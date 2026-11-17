package astisub

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// http://docs.aegisub.org/latest/ASS_Tags/#transform

// A \t transform animates override tags over a time interval relative to the start of
// its event: \t([<t1>,<t2>,][<accel>,]<style modifiers>). The times are milliseconds,
// the acceleration any positive float (1 being linear). This provides basic structured
// access to such transforms: the time interval, the acceleration and the color/scale
// target tags are parsed, anything more exotic is reported instead of being silently
// dropped.

// SSA transform errors
var (
	// ErrSSATransformMalformed is returned when a \t transform cannot be parsed:
	// missing or unbalanced parentheses, empty arguments, missing target tags or
	// invalid color/scale values
	ErrSSATransformMalformed = errors.New("astisub: malformed ssa transform")
	// ErrSSATransformInvalidTime is returned when a \t transform carries negative
	// times or an end time smaller than its start time
	ErrSSATransformInvalidTime = errors.New("astisub: invalid ssa transform time")
	// ErrSSATransformInvalidAcceleration is returned when a \t transform carries
	// an acceleration that is not strictly positive
	ErrSSATransformInvalidAcceleration = errors.New("astisub: invalid ssa transform acceleration")
	// ErrSSATransformUnsupportedTag is returned when a \t transform targets a tag
	// this package does not support (anything but colors and scales, e.g. \fs,
	// \clip or a nested \t)
	ErrSSATransformUnsupportedTag = errors.New("astisub: unsupported ssa transform tag")
)

// SSA transform regexps
var (
	ssaRegexpTransformNumber = regexp.MustCompile(`^[-+]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)$`)
)

// SSATextPart represents a piece of an SSA event text: either raw content kept
// verbatim (literal text or override blocks without transforms) or a parsed \t
// transform. Use ParseSSAText to split an event text and WriteSSAText to build it
// back.
type SSATextPart struct {
	Text      string        // raw content, written back as-is
	Transform *SSATransform // set when the part is a \t transform
}

// SSATransform represents a parsed \t(...) transform
type SSATransform struct {
	Start, End   time.Duration     // animation interval relative to the event start, set when HasTime is true
	HasTime      bool              // whether the transform carries an explicit time interval
	Acceleration float64           // animation acceleration, 0 when unspecified (ASS then defaults to 1)
	Tags         []SSATransformTag // target override tags, in order
}

// SSATransformTag represents one target override tag of a transform, e.g. `\fscx150`
// gives a tag named "fscx" with the arguments "150". Only color tags (\c, \1c to \4c)
// and scale tags (\fscx, \fscy) are supported.
type SSATransformTag struct {
	Name string // e.g. "1c" or "fscx"
	Args string // raw arguments, e.g. "&H0000FF&" or "150"
}

// ParseSSAText splits an SSA event text into its parts: literal text and override
// blocks without transforms are kept verbatim, \t transforms are parsed. The parts
// never drop content: a block whose transform cannot be parsed is kept verbatim and
// the first encountered error is returned, so writing the parts back never silently
// swallows text. Note that an unterminated "{" is not an override block and remains
// literal text, just like in the event parser.
func ParseSSAText(t string) (o []SSATextPart, err error) {
	var previous int
	for _, idxs := range ssaRegexpOverrideBlock.FindAllStringIndex(t, -1) {
		if idxs[0] > previous {
			o = append(o, SSATextPart{Text: t[previous:idxs[0]]})
		}
		parts, perr := splitSSATransformBlock(t[idxs[0]:idxs[1]])
		if perr != nil && err == nil {
			err = perr
		}
		o = append(o, parts...)
		previous = idxs[1]
	}
	if previous < len(t) {
		o = append(o, SSATextPart{Text: t[previous:]})
	}
	return
}

// splitSSATransformBlock splits an override block into its raw runs and its \t
// transforms, keeping their order. A block without transforms is kept verbatim. If a
// transform cannot be parsed, the whole block is kept verbatim so nothing is lost.
func splitSSATransformBlock(block string) (o []SSATextPart, err error) {
	var inner = block[1 : len(block)-1]
	var matches = ssaRegexpOverrideTag.FindAllStringSubmatchIndex(inner, -1)

	// No transform: keep the block verbatim
	var hasTransform bool
	for _, m := range matches {
		if inner[m[2]:m[3]] == "t" {
			hasTransform = true
			break
		}
	}
	if !hasTransform {
		return []SSATextPart{{Text: block}}, nil
	}

	// Split the block around its transforms, keeping the raw runs in between
	var runStart int
	var flush = func(end int) {
		if s := inner[runStart:end]; strings.TrimSpace(s) != "" {
			o = append(o, SSATextPart{Text: "{" + s + "}"})
		}
	}
	for _, m := range matches {
		if inner[m[2]:m[3]] != "t" {
			continue
		}
		var t *SSATransform
		if t, err = parseSSATransform(strings.TrimSpace(inner[m[4]:m[5]])); err != nil {
			return []SSATextPart{{Text: block}}, err
		}
		flush(m[0])
		o = append(o, SSATextPart{Transform: t})
		runStart = m[1]
	}
	flush(len(inner))
	return
}

// parseSSATransform parses the arguments of a \t tag, parentheses included, e.g.
// "(0,1000,1,\1c&H0000FF&)"
func parseSSATransform(args string) (t *SSATransform, err error) {
	// Arguments must be parenthesized
	if len(args) < 2 || args[0] != '(' || args[len(args)-1] != ')' {
		return nil, fmt.Errorf("%w: arguments must be parenthesized, got %q", ErrSSATransformMalformed, args)
	}

	// Leading numeric arguments are the times and the acceleration: with one number
	// it is the acceleration, with two the times, with three both
	t = &SSATransform{}
	var segments = strings.Split(args[1:len(args)-1], ",")
	var numbers []float64
	var idx int
	for idx < len(segments) && len(numbers) < 3 {
		var s = strings.TrimSpace(segments[idx])
		if !ssaRegexpTransformNumber.MatchString(s) {
			break
		}
		var v float64
		if v, err = strconv.ParseFloat(s, 64); err != nil {
			return nil, fmt.Errorf("%w: parsing number %q failed: %w", ErrSSATransformMalformed, s, err)
		}
		numbers = append(numbers, v)
		idx++
	}
	if len(numbers) == 3 && idx < len(segments) && ssaRegexpTransformNumber.MatchString(strings.TrimSpace(segments[idx])) {
		return nil, fmt.Errorf("%w: too many numeric arguments in %q", ErrSSATransformMalformed, args)
	}
	switch len(numbers) {
	case 1:
		if err = t.setAcceleration(numbers[0]); err != nil {
			return nil, err
		}
	case 2:
		if err = t.setTime(numbers[0], numbers[1]); err != nil {
			return nil, err
		}
	case 3:
		if err = t.setTime(numbers[0], numbers[1]); err != nil {
			return nil, err
		}
		if err = t.setAcceleration(numbers[2]); err != nil {
			return nil, err
		}
	}

	// Remaining segments hold the target tags
	for ; idx < len(segments); idx++ {
		var tags []SSATransformTag
		if tags, err = parseSSATransformTags(strings.TrimSpace(segments[idx])); err != nil {
			return nil, err
		}
		t.Tags = append(t.Tags, tags...)
	}
	if len(t.Tags) == 0 {
		return nil, fmt.Errorf("%w: no target tag in %q", ErrSSATransformMalformed, args)
	}
	return
}

// setTime sets the transform interval from millisecond values
func (t *SSATransform) setTime(start, end float64) error {
	if start < 0 || end < 0 || start > end {
		return fmt.Errorf("%w: %v,%v", ErrSSATransformInvalidTime, start, end)
	}
	t.Start = time.Duration(start * float64(time.Millisecond))
	t.End = time.Duration(end * float64(time.Millisecond))
	t.HasTime = true
	return nil
}

// setAcceleration sets the transform acceleration
func (t *SSATransform) setAcceleration(a float64) error {
	if a <= 0 {
		return fmt.Errorf("%w: %v", ErrSSATransformInvalidAcceleration, a)
	}
	t.Acceleration = a
	return nil
}

// parseSSATransformTags parses the target tags of a transform segment, e.g.
// "\1c&H0000FF&\fscx150". Only color and scale tags are supported.
func parseSSATransformTags(s string) (o []SSATransformTag, err error) {
	var previous int
	for _, m := range ssaRegexpOverrideTag.FindAllStringSubmatchIndex(s, -1) {
		// Anything but whitespace between tags is invalid
		if strings.TrimSpace(s[previous:m[0]]) != "" {
			return nil, fmt.Errorf("%w: invalid target tag near %q", ErrSSATransformMalformed, s[previous:])
		}
		var tag = SSATransformTag{Name: s[m[2]:m[3]], Args: strings.TrimSpace(s[m[4]:m[5]])}
		if err = tag.validate(); err != nil {
			return nil, err
		}
		o = append(o, tag)
		previous = m[1]
	}
	if len(o) == 0 || strings.TrimSpace(s[previous:]) != "" {
		return nil, fmt.Errorf("%w: invalid target tag %q", ErrSSATransformMalformed, s)
	}
	return
}

// validate checks that the tag is a supported color or scale tag with valid arguments
func (t SSATransformTag) validate() (err error) {
	switch t.Name {
	case "c", "1c", "2c", "3c", "4c":
		if t.Color() == nil {
			return fmt.Errorf("%w: invalid color arguments %q", ErrSSATransformMalformed, t.Args)
		}
	case "fscx", "fscy":
		if _, ok := t.Scale(); !ok {
			return fmt.Errorf("%w: invalid scale arguments %q", ErrSSATransformMalformed, t.Args)
		}
	case "t":
		return fmt.Errorf("%w: nested transforms", ErrSSATransformUnsupportedTag)
	default:
		return fmt.Errorf("%w: \\%s", ErrSSATransformUnsupportedTag, t.Name)
	}
	return
}

// Color returns the color of a color tag (\c, \1c to \4c), nil otherwise
func (t SSATransformTag) Color() *Color {
	switch t.Name {
	case "c", "1c", "2c", "3c", "4c":
	default:
		return nil
	}
	return ssaOverrideColor(t.Args, nil)
}

// Scale returns the percentage of a scale tag (\fscx, \fscy), false otherwise
func (t SSATransformTag) Scale() (float64, bool) {
	switch t.Name {
	case "fscx", "fscy":
	default:
		return 0, false
	}
	if !ssaRegexpTransformNumber.MatchString(t.Args) {
		return 0, false
	}
	v, err := strconv.ParseFloat(t.Args, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// NewSSAColorTransformTag builds a color target tag, e.g. "\1c&H0000FF&" for a red
// "1c" tag
func NewSSAColorTransformTag(name string, c *Color) SSATransformTag {
	return SSATransformTag{
		Name: name,
		Args: fmt.Sprintf("&H%.2X%.2X%.2X&", c.Blue, c.Green, c.Red),
	}
}

// NewSSAScaleTransformTag builds a scale target tag, e.g. "\fscx150" for a 150% "fscx" tag
func NewSSAScaleTransformTag(name string, v float64) SSATransformTag {
	return SSATransformTag{
		Name: name,
		Args: strconv.FormatFloat(v, 'f', -1, 64),
	}
}

// SSATransforms returns the transforms of the parts, in order
func SSATransforms(parts []SSATextPart) (o []*SSATransform) {
	for _, p := range parts {
		if p.Transform != nil {
			o = append(o, p.Transform)
		}
	}
	return
}

// WriteSSAText builds an SSA event text back from its parts. Raw parts are written
// as-is and transforms are written as their own override blocks, producing valid ASS.
func WriteSSAText(parts []SSATextPart) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Transform != nil {
			sb.WriteString(p.Transform.String())
		} else {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// String returns the transform as an ASS override block, e.g. {\t(0,1000,1,\fscx150)}
func (t *SSATransform) String() string {
	var sb strings.Builder
	sb.WriteString(`{\t(`)
	if t.HasTime {
		sb.WriteString(formatSSATransformTime(t.Start))
		sb.WriteString(",")
		sb.WriteString(formatSSATransformTime(t.End))
		sb.WriteString(",")
	}
	if t.Acceleration != 0 {
		sb.WriteString(strconv.FormatFloat(t.Acceleration, 'f', -1, 64))
		sb.WriteString(",")
	}
	for _, tag := range t.Tags {
		sb.WriteString("\\")
		sb.WriteString(tag.Name)
		sb.WriteString(tag.Args)
	}
	sb.WriteString(")}")
	return sb.String()
}

// formatSSATransformTime formats a transform time in milliseconds
func formatSSATransformTime(d time.Duration) string {
	if d%time.Millisecond == 0 {
		return strconv.FormatInt(int64(d/time.Millisecond), 10)
	}
	return strconv.FormatFloat(float64(d)/float64(time.Millisecond), 'f', -1, 64)
}
