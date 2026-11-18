package astisub_test

import (
	"errors"
	"testing"
	"time"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConflictItem(startAt, endAt time.Duration, text string, r *astisub.Region) *astisub.Item {
	return &astisub.Item{
		EndAt:   endAt,
		Lines:   []astisub.Line{{Items: []astisub.LineItem{{Text: text}}}},
		Region:  r,
		StartAt: startAt,
	}
}

func assertItemTiming(t *testing.T, i *astisub.Item, startAt, endAt time.Duration) {
	t.Helper()
	assert.Equal(t, startAt, i.StartAt, "start at of %q", i.String())
	assert.Equal(t, endAt, i.EndAt, "end at of %q", i.String())
}

// TestSubtitles_ResolveConflicts_TwoRegionsThreeGroups checks a two-region,
// three-group fixture: cues in the same group are made sequential and at least
// as long as the minimum duration, cues in different groups keep overlapping,
// and both the change report and the unresolved overlap are filled in.
func TestSubtitles_ResolveConflicts_TwoRegionsThreeGroups(t *testing.T) {
	// Two regions, and a third group made of the cues without a region
	top := &astisub.Region{ID: "top"}
	bottom := &astisub.Region{ID: "bottom"}

	// Items are deliberately interleaved in the slice: the resolver must keep
	// the slice order and only adjust timings.
	a := newConflictItem(0, 4*time.Second, "top A", top)
	d := newConflictItem(0, 5*time.Second, "bottom D", bottom)
	g := newConflictItem(time.Second, 2*time.Second, "free G", nil)
	b := newConflictItem(2*time.Second, 6*time.Second, "top B", top)
	h := newConflictItem(2500*time.Millisecond, 4*time.Second, "free H", nil)
	e := newConflictItem(4500*time.Millisecond, 20*time.Second, "bottom E", bottom)
	c := newConflictItem(5*time.Second, 8*time.Second, "top C", top)
	i := newConflictItem(5*time.Second, 5*time.Second, "free I", nil)
	f := newConflictItem(6*time.Second, 7*time.Second, "bottom F", bottom)

	// Keep style information around to check it survives untouched
	style := &astisub.Style{ID: "style"}
	for _, item := range []*astisub.Item{a, b, c, d, e, f, g, h, i} {
		item.Style = style
	}

	s := &astisub.Subtitles{Items: []*astisub.Item{a, d, g, b, h, e, c, i, f}}
	before := make([]*astisub.Item, len(s.Items))
	copy(before, s.Items)

	// Resolve
	r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{
		MaxShift:    3 * time.Second,
		MinDuration: 2 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, r)

	// The slice order, the text and the style are untouched
	assert.Equal(t, before, s.Items)
	for _, item := range s.Items {
		assert.Equal(t, style, item.Style)
		assert.Len(t, item.Lines, 1)
		assert.NotEmpty(t, item.Lines[0].Items[0].Text)
	}

	// Top region: B is pushed by 2s, C by exactly the 3s MaxShift
	assertItemTiming(t, a, 0, 4*time.Second)
	assertItemTiming(t, b, 4*time.Second, 8*time.Second)
	assertItemTiming(t, c, 8*time.Second, 11*time.Second)

	// Bottom region: E is pushed by 0.5s and blocks F, which cannot move by
	// 14.5s and is left untouched. D and E overlap before resolution only.
	assertItemTiming(t, d, 0, 5*time.Second)
	assertItemTiming(t, e, 5*time.Second, 20500*time.Millisecond)
	assertItemTiming(t, f, 6*time.Second, 7*time.Second)

	// Third group (no region): G is only extended, H is pushed to touch G, I
	// is a zero-duration cue that only gets extended.
	assertItemTiming(t, g, time.Second, 3*time.Second)
	assertItemTiming(t, h, 3*time.Second, 5*time.Second)
	assertItemTiming(t, i, 5*time.Second, 7*time.Second)

	// The last cue of each group is extended, including the cue that ends at
	// the end of the subtitles' original duration

	// Cues from different regions and from the third group keep overlapping
	assert.True(t, a.StartAt < d.EndAt && d.StartAt < a.EndAt, "top A and bottom D overlap")
	assert.True(t, d.StartAt < g.EndAt && g.StartAt < d.EndAt, "bottom D and free G overlap")
	assert.True(t, g.StartAt < a.EndAt && a.StartAt < g.EndAt, "free G and top A overlap")

	// The change report holds every modified cue exactly once, in a
	// deterministic group order
	require.Len(t, r.Changes, 6)
	expectedChanges := []astisub.ConflictChange{
		{Group: "", Item: g, OldStartAt: time.Second, OldEndAt: 2 * time.Second, NewStartAt: time.Second, NewEndAt: 3 * time.Second},
		{Group: "", Item: h, OldStartAt: 2500 * time.Millisecond, OldEndAt: 4 * time.Second, NewStartAt: 3 * time.Second, NewEndAt: 5 * time.Second},
		{Group: "", Item: i, OldStartAt: 5 * time.Second, OldEndAt: 5 * time.Second, NewStartAt: 5 * time.Second, NewEndAt: 7 * time.Second},
		{Group: "bottom", Item: e, OldStartAt: 4500 * time.Millisecond, OldEndAt: 20 * time.Second, NewStartAt: 5 * time.Second, NewEndAt: 20500 * time.Millisecond},
		{Group: "top", Item: b, OldStartAt: 2 * time.Second, OldEndAt: 6 * time.Second, NewStartAt: 4 * time.Second, NewEndAt: 8 * time.Second},
		{Group: "top", Item: c, OldStartAt: 5 * time.Second, OldEndAt: 8 * time.Second, NewStartAt: 8 * time.Second, NewEndAt: 11 * time.Second},
	}
	assert.Equal(t, expectedChanges, r.Changes)

	// The report exposes the shifts
	assert.Equal(t, 2*time.Second, r.Changes[4].StartShift())
	assert.Equal(t, time.Duration(0), r.Changes[0].StartShift())
	assert.Equal(t, time.Second, r.Changes[0].EndShift())

	// The only unresolved overlap is F against E
	assert.True(t, r.HasUnresolved())
	require.Len(t, r.Unresolved, 1)
	assert.Equal(t, astisub.UnresolvedConflict{
		Group:          "bottom",
		Item:           f,
		With:           e,
		OverlapStartAt: 6 * time.Second,
		OverlapEndAt:   7 * time.Second,
		Issue:          astisub.ConflictIssueMaxShift,
	}, r.Unresolved[0])
	assert.Contains(t, r.Unresolved[0].String(), "max shift exceeded")

	// Resolving again is a no-op
	r2, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{
		MaxShift:    3 * time.Second,
		MinDuration: 2 * time.Second,
	})
	require.NoError(t, err)
	assert.Empty(t, r2.Changes)
	assert.Len(t, r2.Unresolved, 1)
}

func TestSubtitles_ResolveConflicts_EdgeCases(t *testing.T) {
	t.Run("zero duration cue at a boundary", func(t *testing.T) {
		z := newConflictItem(0, 0, "zero", nil)
		n := newConflictItem(0, time.Second, "next", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{z, n}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: time.Second})
		require.NoError(t, err)
		assert.Empty(t, r.Changes)
		assert.Empty(t, r.Unresolved)
		assertItemTiming(t, z, 0, 0)
		assertItemTiming(t, n, 0, time.Second)
	})

	t.Run("zero duration cue inside another cue", func(t *testing.T) {
		a := newConflictItem(0, 4*time.Second, "a", nil)
		z := newConflictItem(2*time.Second, 2*time.Second, "zero", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{a, z}}

		// Not enough room to move the zero-duration cue: it is reported
		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: time.Second})
		require.NoError(t, err)
		assert.Empty(t, r.Changes)
		require.Len(t, r.Unresolved, 1)
		assert.Equal(t, time.Duration(2*time.Second), r.Unresolved[0].OverlapStartAt)
		assert.Equal(t, time.Duration(2*time.Second), r.Unresolved[0].OverlapEndAt)
		assertItemTiming(t, z, 2*time.Second, 2*time.Second)

		// Enough room: it is moved to the end of the other cue and stays
		// zero-length
		s = &astisub.Subtitles{Items: []*astisub.Item{a, z}}
		r, err = s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: 2 * time.Second})
		require.NoError(t, err)
		require.Len(t, r.Changes, 1)
		assertItemTiming(t, z, 4*time.Second, 4*time.Second)
	})

	t.Run("same start time keeps the original order", func(t *testing.T) {
		first := newConflictItem(0, 2*time.Second, "first", nil)
		second := newConflictItem(0, 2*time.Second, "second", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: 2 * time.Second})
		require.NoError(t, err)
		require.Len(t, r.Changes, 1)
		assert.Equal(t, second, r.Changes[0].Item)
		assertItemTiming(t, first, 0, 2*time.Second)
		assertItemTiming(t, second, 2*time.Second, 4*time.Second)
		assert.Empty(t, r.Unresolved)
	})

	t.Run("touching boundaries are not an overlap", func(t *testing.T) {
		first := newConflictItem(0, 2*time.Second, "first", nil)
		second := newConflictItem(2*time.Second, 4*time.Second, "second", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{})
		require.NoError(t, err)
		assert.Empty(t, r.Changes)
		assert.Empty(t, r.Unresolved)
		assertItemTiming(t, second, 2*time.Second, 4*time.Second)
	})

	t.Run("shift exactly at max shift is allowed", func(t *testing.T) {
		first := newConflictItem(0, 2*time.Second, "first", nil)
		second := newConflictItem(time.Second, 3*time.Second, "second", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: time.Second})
		require.NoError(t, err)
		require.Len(t, r.Changes, 1)
		assertItemTiming(t, second, 2*time.Second, 4*time.Second)
		assert.Empty(t, r.Unresolved)
	})

	t.Run("unresolved interval is reported in full", func(t *testing.T) {
		first := newConflictItem(0, 10*time.Second, "first", nil)
		second := newConflictItem(4*time.Second, 11*time.Second, "second", nil)
		third := newConflictItem(6*time.Second, 12*time.Second, "third", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second, third}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MaxShift: 2 * time.Second})
		require.NoError(t, err)
		assert.Empty(t, r.Changes)
		require.Len(t, r.Unresolved, 2)

		// The second cue overlaps the first one over its whole 4s-10s span and
		// would have to move by 6s
		assert.Equal(t, astisub.UnresolvedConflict{
			Group:          "",
			Item:           second,
			With:           first,
			OverlapStartAt: 4 * time.Second,
			OverlapEndAt:   10 * time.Second,
			Issue:          astisub.ConflictIssueMaxShift,
		}, r.Unresolved[0])

		// The second cue is still on screen past the first one, so the third
		// cue now conflicts with it and would have to move by 5s
		assert.Equal(t, astisub.UnresolvedConflict{
			Group:          "",
			Item:           third,
			With:           second,
			OverlapStartAt: 6 * time.Second,
			OverlapEndAt:   11 * time.Second,
			Issue:          astisub.ConflictIssueMaxShift,
		}, r.Unresolved[1])
	})

	t.Run("max shift of zero reports every overlap", func(t *testing.T) {
		first := newConflictItem(0, 2*time.Second, "first", nil)
		second := newConflictItem(time.Second, 3*time.Second, "second", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{})
		require.NoError(t, err)
		assert.Empty(t, r.Changes)
		require.Len(t, r.Unresolved, 1)
		assertItemTiming(t, second, time.Second, 3*time.Second)
	})

	t.Run("invalid options leave the subtitles untouched", func(t *testing.T) {
		first := newConflictItem(0, 2*time.Second, "first", nil)
		second := newConflictItem(time.Second, 3*time.Second, "second", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{first, second}}

		for _, o := range []astisub.ConflictResolutionOptions{
			{MaxShift: -time.Second},
			{MinDuration: -time.Second},
		} {
			r, err := s.ResolveConflicts(o)
			assert.Nil(t, r)
			assert.True(t, errors.Is(err, astisub.ErrInvalidConflictResolutionOptions))
		}
		assertItemTiming(t, first, 0, 2*time.Second)
		assertItemTiming(t, second, time.Second, 3*time.Second)
	})

	t.Run("end of the subtitles' duration", func(t *testing.T) {
		// The subtitles' duration is the end of the last item. Extending that
		// item extends the timeline instead of being clipped.
		last := newConflictItem(5*time.Second, 6*time.Second, "last", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{last}}
		require.Equal(t, 6*time.Second, s.Duration())

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MinDuration: 2 * time.Second})
		require.NoError(t, err)
		require.Len(t, r.Changes, 1)
		assertItemTiming(t, last, 5*time.Second, 7*time.Second)
		assert.Equal(t, 7*time.Second, s.Duration())
		assert.Empty(t, r.Unresolved)
	})

	t.Run("cue ending before it starts", func(t *testing.T) {
		bad := newConflictItem(3*time.Second, 2*time.Second, "bad", nil)
		next := newConflictItem(4*time.Second, 5*time.Second, "next", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{bad, next}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{MinDuration: time.Second})
		require.NoError(t, err)
		require.Len(t, r.Changes, 1)
		assert.Equal(t, bad, r.Changes[0].Item)
		assertItemTiming(t, bad, 3*time.Second, 4*time.Second)

		// The fixed cue now touches the following one, which is untouched
		assertItemTiming(t, next, 4*time.Second, 5*time.Second)
		assert.Empty(t, r.Unresolved)
	})

	t.Run("explicit grouping", func(t *testing.T) {
		a := newConflictItem(0, 2*time.Second, "left:a", nil)
		b := newConflictItem(time.Second, 3*time.Second, "right:b", nil)
		c := newConflictItem(1500*time.Millisecond, 2500*time.Millisecond, "left:c", nil)
		s := &astisub.Subtitles{Items: []*astisub.Item{a, b, c}}

		r, err := s.ResolveConflicts(astisub.ConflictResolutionOptions{
			Group:    func(i *astisub.Item) string { return i.String()[:5] },
			MaxShift: 2 * time.Second,
		})
		require.NoError(t, err)

		// left:a and left:c share a group, right:b is independent
		require.Len(t, r.Changes, 1)
		assert.Equal(t, "left:", r.Changes[0].Group)
		assert.Equal(t, c, r.Changes[0].Item)
		assertItemTiming(t, a, 0, 2*time.Second)
		assertItemTiming(t, b, time.Second, 3*time.Second)
		assertItemTiming(t, c, 2*time.Second, 3*time.Second)
		assert.Empty(t, r.Unresolved)
	})
}
