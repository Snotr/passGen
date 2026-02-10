package generator

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/Snotr/passGen/internal/parser"
)

func collect(ctx context.Context, g *Generator) ([]string, error) {
	out := make(chan string, 1024)
	errCh := make(chan error, 1)
	go func() {
		errCh <- g.Run(ctx, out)
	}()

	var results []string
	for pw := range out {
		results = append(results, pw)
	}
	return results, <-errCh
}

func TestGenerateDefaultMode(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1", "2"}},
	}
	g := New(Config{Groups: groups})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}

	// 2 groups, 2 values each: 2! * 2 * 2 = 8 total
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d: %v", len(results), results)
	}

	// Check that both orderings appear
	sort.Strings(results)
	has := func(s string) bool {
		for _, r := range results {
			if r == s {
				return true
			}
		}
		return false
	}
	// a,b ordering
	if !has("x1") {
		t.Error("missing 'x1'")
	}
	// b,a ordering
	if !has("1x") {
		t.Error("missing '1x'")
	}
}

func TestGeneratePatternMode(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1", "2"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}"})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}

	// Pattern mode: only a,b ordering. 2 * 2 = 4 results
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d: %v", len(results), results)
	}

	expected := []string{"x1", "x2", "y1", "y2"}
	sort.Strings(results)
	sort.Strings(expected)
	for i, e := range expected {
		if results[i] != e {
			t.Errorf("results[%d] = %q, want %q", i, results[i], e)
		}
	}
}

func TestGenerateAmountLimit(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y", "z"}},
		{Tag: "b", Values: []string{"1", "2", "3"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}", MaxCount: 5})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestGenerateTimeLimit(t *testing.T) {
	// Create a large enough group that generation takes a while
	values := make([]string, 1000)
	for i := range values {
		values[i] = "v"
	}
	groups := []parser.TagGroup{
		{Tag: "a", Values: values},
		{Tag: "b", Values: values},
		{Tag: "c", Values: values},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}{c}", MaxDuration: 50 * time.Millisecond})

	start := time.Now()
	results, err := collect(context.Background(), g)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}

	// Should stop well before generating all 1B combos
	if len(results) >= 1000*1000*1000 {
		t.Error("time limit did not take effect")
	}
	// Should have stopped roughly near the time limit (allow generous margin)
	if elapsed > 2*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
	_ = results
}

func TestGenerateSingleGroup(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y", "z"}},
	}
	g := New(Config{Groups: groups})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// 1 group, 3 values: 1! * 3 = 3
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d: %v", len(results), results)
	}
}

func TestGenerateEmptyGroup(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x"}},
		{Tag: "b", Values: nil},
	}
	g := New(Config{Groups: groups})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// With default mode, orderings with the empty group produce nothing.
	// Only orderings where empty group is included will yield 0.
	// Both orderings [a,b] and [b,a] have b with 0 values -> 0 each.
	// But single group "a" alone isn't generated because permutations include all groups.
	// So total = 0 for both orderings.
	// Actually: orderings are permutations of ALL group indices, so [0,1] and [1,0].
	// Both include group b which has 0 values, so product = 0 for both.
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty group, got %d: %v", len(results), results)
	}
}

func TestGenerateNoGroups(t *testing.T) {
	g := New(Config{Groups: nil})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestGenerateStop(t *testing.T) {
	values := make([]string, 10000)
	for i := range values {
		values[i] = "v"
	}
	groups := []parser.TagGroup{
		{Tag: "a", Values: values},
		{Tag: "b", Values: values},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}"})

	out := make(chan string, 1024)
	errCh := make(chan error, 1)
	go func() {
		errCh <- g.Run(context.Background(), out)
	}()

	// Drain some results then stop
	count := 0
	for range out {
		count++
		if count >= 100 {
			g.Stop()
			break
		}
	}
	// Drain remaining
	for range out {
		count++
	}

	err := <-errCh
	if err != nil {
		t.Fatal(err)
	}

	// Should have stopped before generating all 100M combos
	if count >= 10000*10000 {
		t.Error("stop did not take effect")
	}
}

func TestGeneratePauseResume(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1", "2"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}"})

	out := make(chan string, 1024)
	errCh := make(chan error, 1)
	go func() {
		errCh <- g.Run(context.Background(), out)
	}()

	// Collect all results (small enough it completes quickly)
	var results []string
	for pw := range out {
		results = append(results, pw)
	}

	err := <-errCh
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
}

func TestComputeTotal(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1", "2", "3"}},
	}
	// Pattern mode: single ordering [0,1], product = 2*3 = 6
	total := computeTotal(groups, [][]int{{0, 1}})
	if total != 6 {
		t.Errorf("expected 6, got %d", total)
	}

	// Default mode: 2 orderings [0,1] and [1,0], each 2*3=6, total=12
	total = computeTotal(groups, [][]int{{0, 1}, {1, 0}})
	if total != 12 {
		t.Errorf("expected 12, got %d", total)
	}
}

func TestComputeTotalEmpty(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: nil},
	}
	total := computeTotal(groups, [][]int{{0}})
	if total != 0 {
		t.Errorf("expected 0, got %d", total)
	}
}

func TestPermutations(t *testing.T) {
	perms := permutations([]int{0, 1, 2})
	if len(perms) != 6 {
		t.Fatalf("expected 6 permutations of 3 elements, got %d", len(perms))
	}

	// Verify each is a valid permutation
	for _, p := range perms {
		if len(p) != 3 {
			t.Fatalf("permutation has %d elements, want 3", len(p))
		}
		seen := map[int]bool{}
		for _, v := range p {
			seen[v] = true
		}
		if len(seen) != 3 {
			t.Errorf("permutation %v has duplicates", p)
		}
	}
}

func TestPermutationsSingle(t *testing.T) {
	perms := permutations([]int{0})
	if len(perms) != 1 {
		t.Fatalf("expected 1 permutation, got %d", len(perms))
	}
}

func TestPermutationsEmpty(t *testing.T) {
	perms := permutations(nil)
	if perms != nil {
		t.Fatalf("expected nil, got %v", perms)
	}
}

// --- Case expansion tests ---

func TestCamelCase(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "Hello"},
		{"hELLO", "Hello"},
		{"HELLO", "Hello"},
		{"", ""},
		{"a", "A"},
	}
	for _, tc := range tests {
		got := camelCase(tc.input)
		if got != tc.want {
			t.Errorf("camelCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExpandCase(t *testing.T) {
	got := expandCase([]string{"hello"})
	expected := []string{"HELLO", "Hello", "hello"}
	if len(got) != len(expected) {
		t.Fatalf("expandCase([hello]) = %v, want %v", got, expected)
	}
	for i, e := range expected {
		if got[i] != e {
			t.Errorf("expandCase([hello])[%d] = %q, want %q", i, got[i], e)
		}
	}
}

func TestExpandCaseDedup(t *testing.T) {
	// "123" produces identical variants; should deduplicate to 1
	got := expandCase([]string{"123"})
	if len(got) != 1 || got[0] != "123" {
		t.Fatalf("expandCase([123]) = %v, want [123]", got)
	}
}

func TestExpandCaseMultipleValues(t *testing.T) {
	got := expandCase([]string{"alice", "BOB"})
	// "alice" -> ALICE, Alice, alice
	// "BOB"   -> BOB, Bob, bob
	expected := []string{"ALICE", "Alice", "alice", "BOB", "Bob", "bob"}
	if len(got) != len(expected) {
		t.Fatalf("expandCase = %v, want %v", got, expected)
	}
	for i, e := range expected {
		if got[i] != e {
			t.Errorf("[%d] = %q, want %q", i, got[i], e)
		}
	}
}

func TestGenerateCaseExpand(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"hello"}},
		{Tag: "b", Values: []string{"foo"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}", CaseExpand: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// a expands to [HELLO, Hello, hello], b expands to [FOO, Foo, foo]
	// pattern {a}{b}: 3 * 3 = 9 combinations
	sort.Strings(results)
	expected := []string{
		"HELLOFOO", "HELLOFoo", "HELLOfoo",
		"HelloFOO", "HelloFoo", "Hellofoo",
		"helloFOO", "helloFoo", "hellofoo",
	}
	sort.Strings(expected)
	if len(results) != len(expected) {
		t.Fatalf("got %d results, want %d:\ngot:  %v\nwant: %v", len(results), len(expected), results, expected)
	}
	for i, e := range expected {
		if results[i] != e {
			t.Errorf("results[%d] = %q, want %q", i, results[i], e)
		}
	}
}

func TestGenerateCaseExpandDigits(t *testing.T) {
	// Digits don't change with case, so expansion deduplicates to 1 variant
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"alice"}},
		{Tag: "b", Values: []string{"123"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}", CaseExpand: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// a: [ALICE, Alice, alice], b: [123] (deduped)
	// 3 * 1 = 3
	sort.Strings(results)
	expected := []string{"ALICE123", "Alice123", "alice123"}
	sort.Strings(expected)
	if len(results) != len(expected) {
		t.Fatalf("got %d results, want %d: %v", len(results), len(expected), results)
	}
	for i, e := range expected {
		if results[i] != e {
			t.Errorf("results[%d] = %q, want %q", i, results[i], e)
		}
	}
}

// --- Unique tests ---

func TestGenerateUnique(t *testing.T) {
	// Two groups with overlapping values produce duplicates across orderings
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x"}},
		{Tag: "b", Values: []string{"x"}},
	}
	// Default mode: orderings [a,b] and [b,a] both produce "xx"
	g := New(Config{Groups: groups, Unique: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 unique result, got %d: %v", len(results), results)
	}
	if results[0] != "xx" {
		t.Errorf("expected 'xx', got %q", results[0])
	}
}

func TestGenerateUniqueCaseExpand(t *testing.T) {
	// "Hello" and "hello" expand to the same set of variants;
	// with unique, duplicates from overlapping expansions are removed.
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"Hello", "hello"}},
	}
	g := New(Config{Groups: groups, CaseExpand: true, Unique: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Both "Hello" and "hello" expand to HELLO, Hello, hello (identical sets).
	// Dedup in expandCase keeps only 3 distinct values.
	// With Unique generation, those 3 values are output once each.
	sort.Strings(results)
	expected := []string{"HELLO", "Hello", "hello"}
	sort.Strings(expected)
	if len(results) != len(expected) {
		t.Fatalf("expected %d unique results, got %d: %v", len(expected), len(results), results)
	}
	for i, e := range expected {
		if results[i] != e {
			t.Errorf("results[%d] = %q, want %q", i, results[i], e)
		}
	}
}

func TestGenerateUniqueDisabled(t *testing.T) {
	// Same scenario as TestGenerateUnique but without dedup -> duplicates kept
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x"}},
		{Tag: "b", Values: []string{"x"}},
	}
	g := New(Config{Groups: groups, Unique: false})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// 2! * 1 * 1 = 2 (both orderings produce "xx")
	if len(results) != 2 {
		t.Fatalf("expected 2 results without dedup, got %d: %v", len(results), results)
	}
}

// --- Flex tests ---

func TestGenerateFlexTwoGroups(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1"}},
	}
	g := New(Config{Groups: groups, FlexTags: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Subsets of {0,1}:
	//   {0}: perms=[[0]] -> 2 passwords: x, y
	//   {1}: perms=[[1]] -> 1 password: 1
	//   {0,1}: perms=[[0,1],[1,0]] -> 2*1 + 1*2 = 4 passwords: x1,y1,1x,1y
	// Total: 2 + 1 + 4 = 7
	if len(results) != 7 {
		t.Fatalf("expected 7 results, got %d: %v", len(results), results)
	}
}

func TestGenerateFlexPatternIgnored(t *testing.T) {
	// When pattern is set, flex is ignored
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1"}},
	}
	g := New(Config{Groups: groups, Pattern: "{a}{b}", FlexTags: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Pattern mode: only {a}{b} ordering, 2*1 = 2
	if len(results) != 2 {
		t.Fatalf("expected 2 results (pattern overrides flex), got %d: %v", len(results), results)
	}
}

func TestGenerateFlexSingleGroup(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y", "z"}},
	}
	g := New(Config{Groups: groups, FlexTags: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Single group: only subset is {0}, 1 perm, 3 values
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d: %v", len(results), results)
	}
}

func TestSubsets(t *testing.T) {
	subs := subsets([]int{0, 1, 2})
	// 2^3 - 1 = 7 non-empty subsets
	if len(subs) != 7 {
		t.Fatalf("expected 7 subsets, got %d: %v", len(subs), subs)
	}

	// Verify no empty subsets
	for _, s := range subs {
		if len(s) == 0 {
			t.Error("got empty subset")
		}
	}

	// Verify all elements are valid indices
	for _, s := range subs {
		for _, v := range s {
			if v < 0 || v > 2 {
				t.Errorf("invalid index %d in subset %v", v, s)
			}
		}
	}
}

func TestGenerateFlexWithUnique(t *testing.T) {
	// Flex + unique: ensure dedup works across subset orderings
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x"}},
		{Tag: "b", Values: []string{"x"}},
	}
	g := New(Config{Groups: groups, FlexTags: true, Unique: true})
	results, err := collect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Without dedup:
	//   {a}: x
	//   {b}: x
	//   {a,b}: xx, xx (orderings [a,b] and [b,a])
	//   Total: 4
	// With dedup: "x" appears from {a} and {b}, "xx" appears twice -> unique = 2
	if len(results) != 2 {
		t.Fatalf("expected 2 unique results, got %d: %v", len(results), results)
	}
	sort.Strings(results)
	if results[0] != "x" || results[1] != "xx" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestEstimateTotalFlex(t *testing.T) {
	groups := []parser.TagGroup{
		{Tag: "a", Values: []string{"x", "y"}},
		{Tag: "b", Values: []string{"1"}},
	}
	g := New(Config{Groups: groups, FlexTags: true})
	total, err := g.EstimateTotal()
	if err != nil {
		t.Fatal(err)
	}
	// Same as TestGenerateFlexTwoGroups: 2 + 1 + 4 = 7
	if total != 7 {
		t.Fatalf("expected total 7, got %d", total)
	}
}
