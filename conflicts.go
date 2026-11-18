package astisub

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrInvalidConflictResolutionOptions is returned by
// Subtitles.ResolveConflicts when ConflictResolutionOptions holds a negative
// MaxShift or MinDuration.
var ErrInvalidConflictResolutionOptions = errors.New("astisub: invalid conflict resolution options")

// ConflictResolutionOptions configures Subtitles.ResolveConflicts.
//
// Cues are grouped by screen area. By default the group key is the cue's
// region ID and cues without a region belong to a single default group whose
// key is the empty string. Group overrides this. Cues in different groups are
// never compared, so they may overlap freely.
type ConflictResolutionOptions struct {
	// MaxShift is the maximum amount by which a cue may be pushed later to
	// clear an overlap. A cue that would need to move further keeps its
	// original timing and is reported as unresolved.
	MaxShift time.Duration

	// MinDuration is the minimum display duration of a cue. Shorter cues are
	// extended, which may push the following cues of the same group. The
	// extension is not bounded by MaxShift: MaxShift only limits displacement
	// caused by an overlap.
	MinDuration time.Duration

	// Group returns the group key of an item. When nil, the item's region ID
	// is used, or the empty string when the item has no region.
	Group func(i *Item) string
}

func (o ConflictResolutionOptions) validate() error {
	if o.MaxShift < 0 {
		return fmt.Errorf("%w: max shift %s is negative", ErrInvalidConflictResolutionOptions, o.MaxShift)
	}
	if o.MinDuration < 0 {
		return fmt.Errorf("%w: min duration %s is negative", ErrInvalidConflictResolutionOptions, o.MinDuration)
	}
	return nil
}

// groupKey returns the group an item belongs to
func (o ConflictResolutionOptions) groupKey(i *Item) string {
	if o.Group != nil {
		return o.Group(i)
	}
	if i.Region != nil {
		return i.Region.ID
	}
	return ""
}

// ConflictChange reports a cue whose timing was changed by ResolveConflicts.
type ConflictChange struct {
	// Group is the group the cue belongs to.
	Group string

	// Item is the cue whose timing was changed.
	Item *Item

	// OldStartAt and OldEndAt are the cue's timing before resolution.
	OldStartAt time.Duration
	OldEndAt   time.Duration

	// NewStartAt and NewEndAt are the cue's timing after resolution.
	NewStartAt time.Duration
	NewEndAt   time.Duration
}

// StartShift returns how much the cue's start time was pushed later.
func (c ConflictChange) StartShift() time.Duration { return c.NewStartAt - c.OldStartAt }

// EndShift returns how much the cue's end time was pushed later.
func (c ConflictChange) EndShift() time.Duration { return c.NewEndAt - c.OldEndAt }

// String implements the Stringer interface
func (c ConflictChange) String() string {
	return fmt.Sprintf("astisub: group %q: %s-%s -> %s-%s", c.Group,
		c.OldStartAt, c.OldEndAt, c.NewStartAt, c.NewEndAt)
}

// ConflictIssue explains why ResolveConflicts could not fix an overlap.
type ConflictIssue uint8

const (
	// ConflictIssueMaxShift means clearing the overlap would have pushed the
	// cue later than ConflictResolutionOptions.MaxShift allows.
	ConflictIssueMaxShift ConflictIssue = iota + 1
)

// String implements the Stringer interface
func (i ConflictIssue) String() string {
	switch i {
	case ConflictIssueMaxShift:
		return "max shift exceeded"
	default:
		return fmt.Sprintf("unknown conflict issue %d", uint8(i))
	}
}

// UnresolvedConflict reports an overlap that ResolveConflicts left in place.
type UnresolvedConflict struct {
	// Group is the group the two cues belong to.
	Group string

	// Item is the cue that was left untouched. With is the cue whose end time
	// the cue starts before, i.e. the cue it overlaps.
	Item *Item
	With *Item

	// OverlapStartAt and OverlapEndAt delimit the remaining overlap. They are
	// equal when Item has a zero duration.
	OverlapStartAt time.Duration
	OverlapEndAt   time.Duration

	// Issue explains why the overlap was not resolved.
	Issue ConflictIssue
}

// String implements the Stringer interface
func (c UnresolvedConflict) String() string {
	return fmt.Sprintf("astisub: group %q: overlap %s-%s left in place: %s", c.Group,
		c.OverlapStartAt, c.OverlapEndAt, c.Issue)
}

// ConflictResolution is the report returned by ResolveConflicts.
type ConflictResolution struct {
	// Changes lists every cue whose start or end time was modified.
	Changes []ConflictChange

	// Unresolved lists every overlap that was left in place.
	Unresolved []UnresolvedConflict
}

// HasUnresolved returns whether at least one overlap was left in place.
func (r ConflictResolution) HasUnresolved() bool { return len(r.Unresolved) > 0 }

// ResolveConflicts removes, where possible, the time overlaps between cues
// that share the same screen area.
//
// Items are grouped by region ID (or by ConflictResolutionOptions.Group) and
// each group is processed independently, so cues in different regions keep
// overlapping freely. Within a group, cues are considered in chronological
// order; cues with the same start time keep their relative order. A cue that
// starts before the end of the previous one is pushed so that it starts at
// that end, and cues shorter than MinDuration are extended. A cue that would
// have to move by more than MaxShift is left untouched and reported in
// ConflictResolution.Unresolved.
//
// The Items slice itself is never reordered and cue text, lines and styles are
// never modified. An item whose end precedes its start is treated as
// zero-duration and gets a valid timing. Invalid options return
// ErrInvalidConflictResolutionOptions and leave the subtitles untouched.
func (s *Subtitles) ResolveConflicts(o ConflictResolutionOptions) (*ConflictResolution, error) {
	// Validate options before touching anything
	if err := o.validate(); err != nil {
		return nil, err
	}

	// Init report
	r := &ConflictResolution{}

	// Group items, preserving their original order within each group
	groups := make(map[string][]int)
	for idx, i := range s.Items {
		key := o.groupKey(i)
		groups[key] = append(groups[key], idx)
	}

	// Process groups in a deterministic order
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Process groups
	for _, key := range keys {
		indices := groups[key]

		// Consider cues chronologically, keeping the original order for equal
		// start times. The Items slice is not reordered.
		sort.SliceStable(indices, func(a, b int) bool {
			return s.Items[indices[a]].StartAt < s.Items[indices[b]].StartAt
		})

		// blocker is the placed cue that reaches furthest, endAt its end
		var blocker *Item
		var endAt time.Duration
		for _, idx := range indices {
			i := s.Items[idx]
			oldStartAt, oldEndAt := i.StartAt, i.EndAt

			// A cue must not end before it starts. Its original end is used
			// when deciding what it blocks, since an unresolved cue is left
			// untouched.
			originalEndAt := oldEndAt
			if originalEndAt < oldStartAt {
				originalEndAt = oldStartAt
			}

			// Compute the new timing: clear the previous cue and enforce the
			// minimum duration
			d := originalEndAt - oldStartAt
			if d < o.MinDuration {
				d = o.MinDuration
			}
			newStartAt := oldStartAt
			if blocker != nil && newStartAt < endAt {
				newStartAt = endAt
			}
			newEndAt := newStartAt + d

			// The cue cannot be moved enough: leave it untouched and report
			// the remaining overlap
			if newStartAt-oldStartAt > o.MaxShift {
				overlapEndAt := originalEndAt
				if endAt < overlapEndAt {
					overlapEndAt = endAt
				}
				r.Unresolved = append(r.Unresolved, UnresolvedConflict{
					Group:          key,
					Item:           i,
					With:           blocker,
					OverlapStartAt: oldStartAt,
					OverlapEndAt:   overlapEndAt,
					Issue:          ConflictIssueMaxShift,
				})

				// The cue keeps its timing, it only becomes the blocker when it
				// reaches further than the current one
				if originalEndAt > endAt {
					endAt = originalEndAt
					blocker = i
				}
				continue
			}

			// Apply the new timing
			if newStartAt != oldStartAt || newEndAt != oldEndAt {
				i.StartAt = newStartAt
				i.EndAt = newEndAt
				r.Changes = append(r.Changes, ConflictChange{
					Group:      key,
					Item:       i,
					OldStartAt: oldStartAt,
					OldEndAt:   oldEndAt,
					NewStartAt: newStartAt,
					NewEndAt:   newEndAt,
				})
			}

			// Update the blocker
			endAt = newEndAt
			blocker = i
		}
	}
	return r, nil
}
