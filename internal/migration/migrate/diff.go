package migrate

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

type lineStep struct {
	kind byte // ' ', '-', '+'
	text string
}

const (
	diffContext = 3
	maxLCSCells = 200000
)

// WriteDiff renders a deterministic, dependency-free unified diff. It never
// evaluates HCL and does not call external diff executables.
func WriteDiff(writer io.Writer, changes []rewrite.Change) error {
	ordered := append([]rewrite.Change(nil), changes...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, change := range ordered {
		if bytes.Equal(change.Before, change.After) {
			continue
		}
		before := splitLines(string(change.Before))
		after := splitLines(string(change.After))
		steps := diffLines(before, after)
		if len(steps) == 0 {
			continue
		}
		oldPath := "a/" + change.Path
		if !change.Exists {
			oldPath = "/dev/null"
		}
		if _, err := fmt.Fprintf(writer, "--- %s\n+++ b/%s\n", oldPath, change.Path); err != nil {
			return err
		}
		if err := writeHunks(writer, steps); err != nil {
			return err
		}
	}
	return nil
}

func splitLines(source string) []string {
	if source == "" {
		return nil
	}
	result := strings.SplitAfter(source, "\n")
	if result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}

func diffLines(before, after []string) []lineStep {
	var result []lineStep
	var process func([]string, []string)
	process = func(a, b []string) {
		for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
			result = append(result, lineStep{kind: ' ', text: a[0]})
			a, b = a[1:], b[1:]
		}
		suffix := 0
		for suffix < len(a) && suffix < len(b) &&
			a[len(a)-1-suffix] == b[len(b)-1-suffix] {
			suffix++
		}
		common := a[len(a)-suffix:]
		a, b = a[:len(a)-suffix], b[:len(b)-suffix]

		switch {
		case len(a) == 0:
			for _, line := range b {
				result = append(result, lineStep{kind: '+', text: line})
			}
		case len(b) == 0:
			for _, line := range a {
				result = append(result, lineStep{kind: '-', text: line})
			}
		case len(a) <= maxLCSCells/len(b):
			appendLCS(&result, a, b)
		default:
			anchors := uniqueAnchors(a, b)
			if len(anchors) == 0 {
				// Bound memory for very large wholly replaced sections.
				for _, line := range a {
					result = append(result, lineStep{kind: '-', text: line})
				}
				for _, line := range b {
					result = append(result, lineStep{kind: '+', text: line})
				}
			} else {
				left, right := 0, 0
				for _, anchor := range anchors {
					process(a[left:anchor[0]], b[right:anchor[1]])
					result = append(result, lineStep{kind: ' ', text: a[anchor[0]]})
					left, right = anchor[0]+1, anchor[1]+1
				}
				process(a[left:], b[right:])
			}
		}
		for _, line := range common {
			result = append(result, lineStep{kind: ' ', text: line})
		}
	}
	process(before, after)
	return result
}

func appendLCS(result *[]lineStep, a, b []string) {
	width := len(b) + 1
	dp := make([]int, (len(a)+1)*width)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			idx := i*width + j
			switch {
			case a[i] == b[j]:
				dp[idx] = 1 + dp[(i+1)*width+j+1]
			case dp[(i+1)*width+j] >= dp[i*width+j+1]:
				dp[idx] = dp[(i+1)*width+j]
			default:
				dp[idx] = dp[i*width+j+1]
			}
		}
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i == len(a):
			*result = append(*result, lineStep{kind: '+', text: b[j]})
			j++
		case j == len(b):
			*result = append(*result, lineStep{kind: '-', text: a[i]})
			i++
		case a[i] == b[j]:
			*result = append(*result, lineStep{kind: ' ', text: a[i]})
			i++
			j++
		case dp[(i+1)*width+j] >= dp[i*width+j+1]:
			*result = append(*result, lineStep{kind: '-', text: a[i]})
			i++
		default:
			*result = append(*result, lineStep{kind: '+', text: b[j]})
			j++
		}
	}
}

// Use uniquely occurring shared lines as stable anchors when the exact LCS
// would require excessive memory. LIS preserves file order across anchors.
func uniqueAnchors(a, b []string) [][2]int {
	aPositions := make(map[string]int, len(a))
	bPositions := make(map[string]int, len(b))
	for i, line := range a {
		if _, found := aPositions[line]; found {
			aPositions[line] = -1
		} else {
			aPositions[line] = i
		}
	}
	for i, line := range b {
		if _, found := bPositions[line]; found {
			bPositions[line] = -1
		} else {
			bPositions[line] = i
		}
	}
	var pairs [][2]int
	for line, ai := range aPositions {
		if bi, ok := bPositions[line]; ai >= 0 && ok && bi >= 0 {
			pairs = append(pairs, [2]int{ai, bi})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	if len(pairs) == 0 {
		return nil
	}
	tails := make([]int, 0, len(pairs))
	previous := make([]int, len(pairs))
	for i, pair := range pairs {
		pos := sort.Search(len(tails), func(j int) bool {
			return pairs[tails[j]][1] >= pair[1]
		})
		previous[i] = -1
		if pos > 0 {
			previous[i] = tails[pos-1]
		}
		if pos == len(tails) {
			tails = append(tails, i)
		} else {
			tails[pos] = i
		}
	}
	anchors := make([][2]int, len(tails))
	cursor := tails[len(tails)-1]
	for i := len(anchors) - 1; i >= 0; i-- {
		anchors[i] = pairs[cursor]
		cursor = previous[cursor]
	}
	return anchors
}

func writeHunks(writer io.Writer, steps []lineStep) error {
	var changed []int
	for i, step := range steps {
		if step.kind != ' ' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	oldPositions := make([]int, len(steps)+1)
	newPositions := make([]int, len(steps)+1)
	for i, step := range steps {
		oldPositions[i+1], newPositions[i+1] = oldPositions[i], newPositions[i]
		if step.kind != '+' {
			oldPositions[i+1]++
		}
		if step.kind != '-' {
			newPositions[i+1]++
		}
	}
	for index := 0; index < len(changed); {
		start := max(0, changed[index]-diffContext)
		last := changed[index]
		index++
		for index < len(changed) && changed[index]-last <= 2*diffContext+1 {
			last = changed[index]
			index++
		}
		end := min(len(steps), last+diffContext+1)
		oldCount := oldPositions[end] - oldPositions[start]
		newCount := newPositions[end] - newPositions[start]
		oldStart := oldPositions[start]
		newStart := newPositions[start]
		if oldCount != 0 {
			oldStart++
		}
		if newCount != 0 {
			newStart++
		}
		if _, err := fmt.Fprintf(writer, "@@ -%d,%d +%d,%d @@\n",
			oldStart, oldCount, newStart, newCount); err != nil {
			return err
		}
		for _, step := range steps[start:end] {
			if _, err := fmt.Fprintf(writer, "%c%s", step.kind, step.text); err != nil {
				return err
			}
			if !strings.HasSuffix(step.text, "\n") {
				if _, err := fmt.Fprint(writer, "\n\\ No newline at end of file\n"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
