// Package generator produces passwords by combining tag groups using
// configurable orderings, case expansion, deduplication, and limits.
package generator

import (
	"context"
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Snotr/passGen/internal/parser"
)

// Config controls how the generator behaves.
type Config struct {
	Groups      []parser.TagGroup
	Pattern     string
	MaxCount    int64
	MaxDuration time.Duration
	CaseExpand  bool // triplicate values into UPPER, Camel, lower variants
	Unique      bool
	FlexTags    bool
}

// State represents the generator's runtime state.
type State int32

const (
	StateRunning State = iota
	StatePaused
	StateStopped
	StateDone
)

// Progress holds generation metrics, safe for concurrent reads.
type Progress struct {
	generated atomic.Int64
	total     int64
	startTime time.Time
	state     atomic.Int32
}

func (p *Progress) Generated() int64       { return p.generated.Load() }
func (p *Progress) Total() int64           { return p.total }
func (p *Progress) Elapsed() time.Duration { return time.Since(p.startTime) }
func (p *Progress) State() State           { return State(p.state.Load()) }
func (p *Progress) setState(s State)       { p.state.Store(int32(s)) }

// Generator produces passwords from TagGroups.
type Generator struct {
	cfg      Config
	Progress *Progress
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
}

// New creates a Generator from the given config.
// When CaseExpand is true, each group's values are tripled into UPPER, Camel,
// and lower variants (deduplicated) before any generation begins.
func New(cfg Config) *Generator {
	if cfg.CaseExpand {
		expanded := make([]parser.TagGroup, len(cfg.Groups))
		for i, g := range cfg.Groups {
			expanded[i] = parser.TagGroup{
				Tag:    g.Tag,
				Values: expandCase(g.Values),
			}
		}
		cfg.Groups = expanded
	}
	return &Generator{
		cfg:      cfg,
		Progress: &Progress{},
		pauseCh:  make(chan struct{}, 1),
		resumeCh: make(chan struct{}, 1),
		stopCh:   make(chan struct{}, 1),
	}
}

// Pause signals the generator to pause.
func (g *Generator) Pause() {
	select {
	case g.pauseCh <- struct{}{}:
	default:
	}
}

// Resume signals the generator to resume from a pause.
func (g *Generator) Resume() {
	select {
	case g.resumeCh <- struct{}{}:
	default:
	}
}

// Stop signals the generator to stop.
func (g *Generator) Stop() {
	select {
	case g.stopCh <- struct{}{}:
	default:
	}
}

// EstimateTotal computes the total number of passwords that would be generated
// without starting generation. Returns -1 on overflow.
func (g *Generator) EstimateTotal() (int64, error) {
	groups := g.cfg.Groups
	if len(groups) == 0 {
		return 0, nil
	}
	orderings, err := g.resolveOrderings()
	if err != nil {
		return 0, err
	}
	return computeTotal(groups, orderings), nil
}

func (g *Generator) resolveOrderings() ([][]int, error) {
	groups := g.cfg.Groups
	groupMap := make(map[string]int, len(groups))
	for i, grp := range groups {
		groupMap[grp.Tag] = i
	}

	var orderings [][]int
	if g.cfg.Pattern != "" {
		tags, err := ParsePattern(g.cfg.Pattern, groups)
		if err != nil {
			return nil, err
		}
		ordering := make([]int, len(tags))
		for i, tag := range tags {
			ordering[i] = groupMap[tag]
		}
		orderings = [][]int{ordering}
	} else if g.cfg.FlexTags {
		// Generate all non-empty subsets; each subset gets all permutations
		indices := make([]int, len(groups))
		for i := range indices {
			indices[i] = i
		}
		for _, sub := range subsets(indices) {
			orderings = append(orderings, permutations(sub)...)
		}
	} else {
		indices := make([]int, len(groups))
		for i := range indices {
			indices[i] = i
		}
		orderings = permutations(indices)
	}
	return orderings, nil
}

// Run starts generation and sends passwords on the out channel.
// It closes out when done. Respects context cancellation, pause/resume, and stop.
func (g *Generator) Run(ctx context.Context, out chan<- string) error {
	defer close(out)

	groups := g.cfg.Groups
	if len(groups) == 0 {
		return nil
	}

	orderings, err := g.resolveOrderings()
	if err != nil {
		return err
	}

	// Compute total
	g.Progress.total = computeTotal(groups, orderings)
	g.Progress.startTime = time.Now()
	g.Progress.setState(StateRunning)

	var seen map[string]struct{}
	if g.cfg.Unique {
		seen = make(map[string]struct{})
	}

	for _, ordering := range orderings {
		if err := g.generateOrdering(ctx, out, groups, ordering, seen); err != nil {
			return err
		}
		if g.Progress.State() == StateStopped {
			return nil
		}
	}

	g.Progress.setState(StateDone)
	return nil
}

func (g *Generator) generateOrdering(ctx context.Context, out chan<- string, groups []parser.TagGroup, ordering []int, seen map[string]struct{}) error {
	n := len(ordering)
	if n == 0 {
		return nil
	}

	// sizes[i] = len(groups[ordering[i]].Values)
	sizes := make([]int, n)
	for i, idx := range ordering {
		sizes[i] = len(groups[idx].Values)
		if sizes[i] == 0 {
			return nil // empty group means no output for this ordering
		}
	}

	indices := make([]int, n)
	var buf strings.Builder
	checkInterval := 0

	for {
		// Periodic control check
		checkInterval++
		if checkInterval >= 1000 {
			checkInterval = 0
			if err := g.checkControls(ctx); err != nil {
				return err
			}
			if g.Progress.State() == StateStopped {
				return nil
			}
		}

		// Check limits
		if g.cfg.MaxCount > 0 && g.Progress.Generated() >= g.cfg.MaxCount {
			g.Progress.setState(StateDone)
			return nil
		}
		if g.cfg.MaxDuration > 0 && g.Progress.Elapsed() >= g.cfg.MaxDuration {
			g.Progress.setState(StateDone)
			return nil
		}

		// Build password
		buf.Reset()
		for i, idx := range ordering {
			buf.WriteString(groups[idx].Values[indices[i]])
		}

		pw := buf.String()

		// Dedup check
		if seen != nil {
			if _, dup := seen[pw]; dup {
				// skip duplicate; still advance counter
				if !advance(indices, sizes) {
					break
				}
				continue
			}
			seen[pw] = struct{}{}
		}

		select {
		case out <- pw:
		case <-ctx.Done():
			return ctx.Err()
		}
		g.Progress.generated.Add(1)

		// Advance mixed-radix counter
		if !advance(indices, sizes) {
			break
		}
	}

	return nil
}

func (g *Generator) checkControls(ctx context.Context) error {
	// Check stop
	select {
	case <-g.stopCh:
		g.Progress.setState(StateStopped)
		return nil
	default:
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check pause
	select {
	case <-g.pauseCh:
		g.Progress.setState(StatePaused)
		// Wait for resume or stop
		select {
		case <-g.resumeCh:
			g.Progress.setState(StateRunning)
		case <-g.stopCh:
			g.Progress.setState(StateStopped)
		case <-ctx.Done():
			return ctx.Err()
		}
	default:
	}

	return nil
}

// advance increments a mixed-radix counter. Returns false when it wraps around.
func advance(indices, sizes []int) bool {
	for i := len(indices) - 1; i >= 0; i-- {
		indices[i]++
		if indices[i] < sizes[i] {
			return true
		}
		indices[i] = 0
	}
	return false
}

// computeTotal calculates the total number of combinations across all orderings.
// Returns -1 on overflow.
func computeTotal(groups []parser.TagGroup, orderings [][]int) int64 {
	var total int64
	for _, ordering := range orderings {
		product := int64(1)
		for _, idx := range ordering {
			sz := int64(len(groups[idx].Values))
			if sz == 0 {
				product = 0
				break
			}
			if product > math.MaxInt64/sz {
				return -1
			}
			product *= sz
		}
		if total > math.MaxInt64-product {
			return -1
		}
		total += product
	}
	return total
}

// expandCase triplicates each value into UPPER, Camel, and lower variants,
// deduplicating while preserving insertion order.
func expandCase(values []string) []string {
	seen := make(map[string]struct{}, len(values)*3)
	expanded := make([]string, 0, len(values)*3)
	for _, v := range values {
		upper := strings.ToUpper(v)
		camel := camelCase(v)
		lower := strings.ToLower(v)
		for _, variant := range []string{upper, camel, lower} {
			if _, dup := seen[variant]; !dup {
				seen[variant] = struct{}{}
				expanded = append(expanded, variant)
			}
		}
	}
	return expanded
}

// camelCase returns s with the first rune uppercased and the rest lowercased.
func camelCase(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + strings.ToLower(s[size:])
}

// subsets returns all non-empty subsets of the given slice, using bitmask enumeration.
func subsets(arr []int) [][]int {
	n := len(arr)
	total := 1 << n // 2^n
	result := make([][]int, 0, total-1)
	for mask := 1; mask < total; mask++ {
		var sub []int
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sub = append(sub, arr[i])
			}
		}
		result = append(result, sub)
	}
	return result
}

// permutations generates all permutations of the input slice using Heap's algorithm.
func permutations(arr []int) [][]int {
	n := len(arr)
	if n == 0 {
		return nil
	}

	var result [][]int
	c := make([]int, n)
	// Add initial permutation
	perm := make([]int, n)
	copy(perm, arr)
	result = append(result, perm)

	i := 0
	for i < n {
		if c[i] < i {
			if i%2 == 0 {
				arr[0], arr[i] = arr[i], arr[0]
			} else {
				arr[c[i]], arr[i] = arr[i], arr[c[i]]
			}
			perm := make([]int, n)
			copy(perm, arr)
			result = append(result, perm)
			c[i]++
			i = 0
		} else {
			c[i] = 0
			i++
		}
	}
	return result
}
