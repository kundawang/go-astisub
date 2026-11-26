package astisub

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

// ASS fade override tags
//
// http://docs.aegisub.org/latest/ASS_Tags/#\fad
// http://docs.aegisub.org/latest/ASS_Tags/#\fade
//
// The \fad and \fade override tags animate the primary alpha of a whole event.
// Alpha is expressed the ASS way: 0 is fully opaque, 255 is fully transparent,
// and the times are milliseconds relative to the start of the event.
const (
	// ssaFadeTagFAD is the \fad tag: a fade in and a fade out at the event edges
	ssaFadeTagFAD = "fad"
	// ssaFadeTagFADE is the \fade tag: three timed alpha levels
	ssaFadeTagFADE = "fade"
)

// ErrInvalidSSAFade is returned when an SSAFade cannot be built or serialized,
// e.g. because its alpha levels are outside the 0-255 range or its times are
// not ordered
var ErrInvalidSSAFade = errors.New("astisub: invalid SSA fade")

// SSAFade represents an ASS \fad or \fade override tag in an editable,
// structured form. Times are relative to the start of the event, as in the
// specification.
//
// For an SSAFadeTypeFAD fade, FadeIn and FadeOut are the fade-in and fade-out
// durations. For an SSAFadeTypeFADE fade, Alpha1/2/3 and Time1/2/3/4 describe
// the three alpha levels and the four transition times of \fade.
type SSAFade struct {
	// Type is SSAFadeTypeFAD or SSAFadeTypeFADE
	Type string

	// FadeIn is the \fad fade-in duration in milliseconds
	FadeIn int
	// FadeOut is the \fad fade-out duration in milliseconds
	FadeOut int

	// Alpha1 is the \fade alpha before Time1 (0-255)
	Alpha1 uint8
	// Alpha2 is the \fade alpha between Time2 and Time3 (0-255)
	Alpha2 uint8
	// Alpha3 is the \fade alpha after Time4 (0-255)
	Alpha3 uint8
	// Time1 is the \fade end of the first transition, in milliseconds
	Time1 int
	// Time2 is the \fade start of the second transition, in milliseconds
	Time2 int
	// Time3 is the \fade end of the second transition, in milliseconds
	Time3 int
	// Time4 is the \fade start of the third transition, in milliseconds
	Time4 int
}

// SSAFade types
const (
	// SSAFadeTypeFAD is the simple two-duration \fad tag
	SSAFadeTypeFAD = "fad"
	// SSAFadeTypeFADE is the seven-argument \fade tag
	SSAFadeTypeFADE = "fade"
)

// NewSSAFADFade builds the structured representation of a \fad(in,out) tag.
// Negative durations are rejected.
func NewSSAFADFade(fadeIn, fadeOut time.Duration) (*SSAFade, error) {
	if fadeIn < 0 || fadeOut < 0 {
		return nil, ErrInvalidSSAFade
	}
	return &SSAFade{
		Type:    SSAFadeTypeFAD,
		FadeIn:  int(fadeIn / time.Millisecond),
		FadeOut: int(fadeOut / time.Millisecond),
	}, nil
}

// NewSSAFADEFade builds the structured representation of a
// \fade(a1,a2,a3,t1,t2,t3,t4) tag. The times must be ordered
// (t1 <= t2 <= t3 <= t4) and non-negative.
func NewSSAFADEFade(alpha1, alpha2, alpha3 uint8, t1, t2, t3, t4 time.Duration) (*SSAFade, error) {
	if t1 < 0 || t2 < 0 || t3 < 0 || t4 < 0 || !(t1 <= t2 && t2 <= t3 && t3 <= t4) {
		return nil, ErrInvalidSSAFade
	}
	return &SSAFade{
		Type:   SSAFadeTypeFADE,
		Alpha1: alpha1,
		Alpha2: alpha2,
		Alpha3: alpha3,
		Time1:  int(t1 / time.Millisecond),
		Time2:  int(t2 / time.Millisecond),
		Time3:  int(t3 / time.Millisecond),
		Time4:  int(t4 / time.Millisecond),
	}, nil
}

// validate reports whether the fade holds legal tag parameters
func (f SSAFade) validate() error {
	switch f.Type {
	case SSAFadeTypeFAD:
		if f.FadeIn < 0 || f.FadeOut < 0 {
			return ErrInvalidSSAFade
		}
	case SSAFadeTypeFADE:
		if f.Time1 < 0 || f.Time2 < 0 || f.Time3 < 0 || f.Time4 < 0 ||
			!(f.Time1 <= f.Time2 && f.Time2 <= f.Time3 && f.Time3 <= f.Time4) {
			return ErrInvalidSSAFade
		}
	default:
		return ErrInvalidSSAFade
	}
	return nil
}

// overrideTag renders the fade as a parsed ssaOverrideTag, e.g.
// `\fad(100,200)` or `\fade(255,0,255,0,100,800,1000)`
func (f SSAFade) overrideTag() (t ssaOverrideTag, err error) {
	if err = f.validate(); err != nil {
		return
	}
	switch f.Type {
	case SSAFadeTypeFAD:
		t = ssaOverrideTag{
			name: ssaFadeTagFAD,
			args: "(" + strconv.Itoa(f.FadeIn) + "," + strconv.Itoa(f.FadeOut) + ")",
		}
	case SSAFadeTypeFADE:
		t = ssaOverrideTag{
			name: ssaFadeTagFADE,
			args: "(" +
				strconv.Itoa(int(f.Alpha1)) + "," +
				strconv.Itoa(int(f.Alpha2)) + "," +
				strconv.Itoa(int(f.Alpha3)) + "," +
				strconv.Itoa(f.Time1) + "," +
				strconv.Itoa(f.Time2) + "," +
				strconv.Itoa(f.Time3) + "," +
				strconv.Itoa(f.Time4) + ")",
		}
	default:
		err = ErrInvalidSSAFade
	}
	return
}

// normalize converts either tag form into the seven boundary values used to
// compute the effective alpha. A \fad(in,out) is equivalent to
// \fade(255,0,255,0,in,duration-out,duration), duration being the event
// duration in milliseconds.
func (f SSAFade) normalize(eventDuration time.Duration) (a1, a2, a3 uint8, t1, t2, t3, t4 int, err error) {
	if err = f.validate(); err != nil {
		return
	}
	switch f.Type {
	case SSAFadeTypeFAD:
		a1, a2, a3 = 255, 0, 255
		t1 = 0
		t2 = f.FadeIn
		var duration = int(eventDuration / time.Millisecond)
		t3 = duration - f.FadeOut
		t4 = duration
	case SSAFadeTypeFADE:
		a1, a2, a3 = f.Alpha1, f.Alpha2, f.Alpha3
		t1, t2, t3, t4 = f.Time1, f.Time2, f.Time3, f.Time4
	}
	return
}

// ssaInterpolateAlpha linearly interpolates an ASS alpha between a and b over
// [start,end], rounding half values away from zero like libass. A zero-length
// transition keeps the starting alpha.
func ssaInterpolateAlpha(a, b uint8, start, end, at int) uint8 {
	if end <= start {
		return a
	}
	if at <= start {
		return a
	}
	if at >= end {
		return b
	}
	var v = float64(a) + float64(int(b)-int(a))*float64(at-start)/float64(end-start)
	return uint8(math.Round(v))
}

// alphaAt returns the effective ASS alpha (0 opaque, 255 transparent) at at,
// a duration relative to the start of the event whose total duration is
// eventDuration. Times before the event use its first alpha level and times
// after it its last one, matching how renderers treat the event boundaries.
func (f SSAFade) alphaAt(at, eventDuration time.Duration) (uint8, error) {
	a1, a2, a3, t1, t2, t3, t4, err := f.normalize(eventDuration)
	if err != nil {
		return 0, err
	}
	var ms = int(at / time.Millisecond)
	switch {
	case ms < t1:
		return a1, nil
	case ms <= t2:
		return ssaInterpolateAlpha(a1, a2, t1, t2, ms), nil
	case ms < t3:
		return a2, nil
	case ms <= t4:
		return ssaInterpolateAlpha(a2, a3, t3, t4, ms), nil
	default:
		return a3, nil
	}
}

// parseSSAFadeTag parses one \fad or \fade override tag. An unparseable tag
// (wrong number of arguments, non-numeric value, out-of-range alpha or
// unordered times) returns an error: callers keep the raw tag untouched so
// invalid content still round-trips verbatim.
func parseSSAFadeTag(tag ssaOverrideTag) (*SSAFade, error) {
	var args = strings.TrimSpace(tag.args)
	args = strings.TrimPrefix(args, "(")
	args = strings.TrimSuffix(args, ")")
	var parts = strings.Split(args, ",")

	switch tag.name {
	case ssaFadeTagFAD:
		if len(parts) != 2 {
			return nil, ErrInvalidSSAFade
		}
		in, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, ErrInvalidSSAFade
		}
		out, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, ErrInvalidSSAFade
		}
		if in < 0 || out < 0 {
			return nil, ErrInvalidSSAFade
		}
		return &SSAFade{Type: SSAFadeTypeFAD, FadeIn: in, FadeOut: out}, nil
	case ssaFadeTagFADE:
		if len(parts) != 7 {
			return nil, ErrInvalidSSAFade
		}
		var values [7]int
		for idx, part := range parts {
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return nil, ErrInvalidSSAFade
			}
			values[idx] = v
		}
		for idx := 0; idx < 3; idx++ {
			if values[idx] < 0 || values[idx] > 255 {
				return nil, ErrInvalidSSAFade
			}
		}
		for idx := 3; idx < 7; idx++ {
			if values[idx] < 0 {
				return nil, ErrInvalidSSAFade
			}
		}
		if !(values[3] <= values[4] && values[4] <= values[5] && values[5] <= values[6]) {
			return nil, ErrInvalidSSAFade
		}
		return &SSAFade{
			Type:   SSAFadeTypeFADE,
			Alpha1: uint8(values[0]),
			Alpha2: uint8(values[1]),
			Alpha3: uint8(values[2]),
			Time1:  values[3],
			Time2:  values[4],
			Time3:  values[5],
			Time4:  values[6],
		}, nil
	}
	return nil, ErrInvalidSSAFade
}

// ssaTagsContainFade reports whether a block's tags include a \fad or \fade tag
func ssaTagsContainFade(tags []ssaOverrideTag) bool {
	for _, tag := range tags {
		if tag.name == ssaFadeTagFAD || tag.name == ssaFadeTagFADE {
			return true
		}
	}
	return false
}

// ssaItemFadeTags returns every valid \fad and \fade tag of the event, in
// document order. Malformed tags are skipped rather than failing the read:
// they remain in the text and round-trip untouched.
func ssaItemFadeTags(i *Item) (o []SSAFade) {
	for _, line := range i.Lines {
		for _, li := range line.Items {
			if li.InlineStyle == nil || li.InlineStyle.SSAEffect == "" {
				continue
			}
			for _, part := range splitSSAOverrides(li.InlineStyle.SSAEffect) {
				if !ssaTagsContainFade(part.tags) {
					continue
				}
				for _, tag := range part.tags {
					if tag.name != ssaFadeTagFAD && tag.name != ssaFadeTagFADE {
						continue
					}
					if f, err := parseSSAFadeTag(tag); err == nil {
						o = append(o, *f)
					}
				}
			}
		}
	}
	return
}

// SSAFades returns the structured representation of every valid \fad and
// \fade override tag of the event, in document order. When the same event
// carries several fade tags, renderers apply the last valid one: SSAFade
// returns that tag directly. Malformed tags are not returned but stay in the
// text untouched.
func (i *Item) SSAFades() []SSAFade {
	return ssaItemFadeTags(i)
}

// SSAFade returns the last valid \fad or \fade override tag of the event, the
// one renderers apply when several are present, and nil when the event has no
// valid fade tag.
func (i *Item) SSAFade() *SSAFade {
	var fades = ssaItemFadeTags(i)
	if len(fades) == 0 {
		return nil
	}
	var f = fades[len(fades)-1]
	return &f
}

// SSAAlphaAt returns the effective ASS alpha level (0 fully opaque, 255 fully
// transparent) of the event at t, an absolute time. Without a valid fade tag
// the event is fully opaque. The fade's relative times are resolved against
// the event's start and end, so the entry, hold and exit phases and their
// endpoints are handled as renderers do.
func (i *Item) SSAAlphaAt(t time.Duration) (uint8, error) {
	var f = i.SSAFade()
	if f == nil {
		return 0, nil
	}
	return f.alphaAt(t-i.StartAt, i.EndAt-i.StartAt)
}

// SetSSAFade replaces every \fad and \fade tag of the event with f, keeping
// the other override tags, the text and the line timing untouched. Pass nil to
// remove all fade tags; an empty override block left behind is removed as
// well. The fade is serialized as the tag form (fad or fade) carried by f.
func (i *Item) SetSSAFade(f *SSAFade) error {
	// Pre-render the replacement tag so an invalid fade never partially mutates
	// the event
	var replacement *ssaOverrideTag
	if f != nil {
		t, err := f.overrideTag()
		if err != nil {
			return err
		}
		replacement = &t
	}

	var inserted bool
	for lineIdx := range i.Lines {
		for itemIdx := range i.Lines[lineIdx].Items {
			var li = &i.Lines[lineIdx].Items[itemIdx]
			if li.InlineStyle == nil || li.InlineStyle.SSAEffect == "" {
				continue
			}

			// Only actual override blocks can carry tags; any other value is
			// left untouched
			var effect = li.InlineStyle.SSAEffect
			if !strings.HasPrefix(effect, "{") || !strings.HasSuffix(effect, "}") {
				continue
			}

			var parts = splitSSAOverrides(effect)
			// Locate the first block holding a fade tag, i.e. the insertion
			// point in document order. When no block carries one, the first
			// tag block is used so the effect is rebuilt unchanged.
			var block *ssaOverridePart
			for idx := range parts {
				if len(parts[idx].tags) == 0 {
					continue
				}
				block = &parts[idx]
				if ssaTagsContainFade(block.tags) {
					break
				}
			}
			if block == nil {
				continue
			}
			var blockHasFade = ssaTagsContainFade(block.tags)

			// Drop every fade tag, keeping the rest in order
			var kept []ssaOverrideTag
			for _, tag := range block.tags {
				if tag.name != ssaFadeTagFAD && tag.name != ssaFadeTagFADE {
					kept = append(kept, tag)
				}
			}

			// The replacement goes into the first block holding a fade tag
			if blockHasFade && !inserted && replacement != nil {
				kept = append([]ssaOverrideTag{*replacement}, kept...)
				inserted = true
			}
			block.tags = kept

			// Rebuild the block; an empty one disappears entirely
			var b strings.Builder
			for _, part := range parts {
				if len(part.tags) > 0 {
					b.WriteString("{")
					for _, tag := range part.tags {
						b.WriteString("\\")
						b.WriteString(tag.name)
						b.WriteString(tag.args)
					}
					b.WriteString("}")
				} else {
					b.WriteString(part.text)
				}
			}
			var rebuilt = b.String()
			if rebuilt == "" {
				li.InlineStyle.SSAEffect = ""
			} else {
				li.InlineStyle.SSAEffect = rebuilt
			}
		}
	}

	// No existing block could host the tag: prepend one to the event
	if replacement != nil && !inserted {
		if len(i.Lines) == 0 || len(i.Lines[0].Items) == 0 {
			return ErrInvalidSSAFade
		}
		var first = &i.Lines[0].Items[0]
		if first.InlineStyle == nil {
			first.InlineStyle = &StyleAttributes{}
		}
		first.InlineStyle.SSAEffect = "{\\" + replacement.name + replacement.args + "}" + first.InlineStyle.SSAEffect
	}
	return nil
}
