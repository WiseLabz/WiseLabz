package doc

// Conflict is a generated block a human edited whose upstream also changed.
// Sync leaves the block alone and raises a review Change instead.
type Conflict struct {
	Key       string
	Human     string // the block's current (edited) body
	Generated Block  // the freshly generated block
}

// Merge folds a fresh render into a doc's existing segments without ever
// discarding human text:
//   - text outside blocks is kept as-is;
//   - an unedited block is refreshed; an edited one is kept, and reported as
//     a Conflict when its upstream changed too;
//   - a block whose key vanished upstream is dropped if unedited, otherwise
//     detached (kept as plain text);
//   - a fresh block is inserted only when its key wasn't in prevKeys (the
//     keys of the last applied render), so a block the user deleted stays
//     deleted. It goes after the nearest preceding fresh block, else at the end.
func Merge(existing []Segment, prevKeys []string, fresh []Block) ([]Segment, []Conflict) {
	freshByKey := make(map[string]Block, len(fresh))
	for _, b := range fresh {
		freshByKey[b.Key] = b
	}
	prev := make(map[string]bool, len(prevKeys))
	for _, k := range prevKeys {
		prev[k] = true
	}

	out := make([]Segment, 0, len(existing)+len(fresh))
	present := map[string]bool{}
	var conflicts []Conflict
	for _, seg := range existing {
		if seg.Block == nil {
			out = append(out, seg)
			continue
		}
		cur := *seg.Block
		nb, ok := freshByKey[cur.Key]
		edited := cur.Edited()
		switch {
		case !ok && !edited:
			continue // gone upstream, nobody touched it
		case !ok:
			out = append(out, Segment{Text: cur.Body}) // detach the human edit
			continue
		case !edited:
			cur = nb
		case nb.Hash != cur.Hash:
			conflicts = append(conflicts, Conflict{Key: cur.Key, Human: cur.Body, Generated: nb})
		}
		present[cur.Key] = true
		out = append(out, Segment{Block: &cur})
	}

	for i, nb := range fresh {
		if present[nb.Key] || prev[nb.Key] {
			continue
		}
		at := len(out)
		for j := i - 1; j >= 0; j-- {
			if idx := blockIndex(out, fresh[j].Key); idx >= 0 {
				at = idx + 1
				break
			}
		}
		b := nb
		ins := []Segment{{Text: "\n\n"}, {Block: &b}}
		if at == len(out) && (len(out) == 0 || endsWithNewline(out)) {
			ins = []Segment{{Block: &b}, {Text: "\n"}}
			if len(out) > 0 {
				ins = append([]Segment{{Text: "\n"}}, ins...)
			}
		}
		out = append(out[:at], append(ins, out[at:]...)...)
		present[nb.Key] = true
	}
	return out, conflicts
}

func blockIndex(segs []Segment, key string) int {
	for i, s := range segs {
		if s.Block != nil && s.Block.Key == key {
			return i
		}
	}
	return -1
}

func endsWithNewline(segs []Segment) bool {
	last := segs[len(segs)-1]
	return last.Block == nil && len(last.Text) > 0 && last.Text[len(last.Text)-1] == '\n'
}
