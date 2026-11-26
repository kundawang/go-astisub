package astisub

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/asticode/go-astikit"
)

// TTML time container values.
// See https://www.w3.org/TR/ttml1/#attribute-timeContainer
const (
	ttmlTimeContainerParallel   = "par"
	ttmlTimeContainerSequential = "seq"
)

// https://www.w3.org/TR/ttaf1-dfxp/
// http://www.skynav.com:8080/ttv/check
// https://www.speechpad.com/captions/ttml

// TTML languages
const (
	ttmlLanguageChinese   = "zh"
	ttmlLanguageEnglish   = "en"
	ttmlLanguageJapanese  = "ja"
	ttmlLanguageFrench    = "fr"
	ttmlLanguageNorwegian = "no"
)

// TTML language mapping
var ttmlLanguageMapping = astikit.NewBiMap().
	Set(ttmlLanguageChinese, LanguageChinese).
	Set(ttmlLanguageEnglish, LanguageEnglish).
	Set(ttmlLanguageFrench, LanguageFrench).
	Set(ttmlLanguageJapanese, LanguageJapanese).
	Set(ttmlLanguageNorwegian, LanguageNorwegian)

// TTML Clock Time Frames and Offset Time
var (
	ttmlRegexpClockTimeFrames = regexp.MustCompile(`\:[\d]+$`)
	ttmlRegexpOffsetTime      = regexp.MustCompile(`^(\d+(\.\d+)?)(h|m|s|ms|f|t)$`)
)

type TTMLInBodyDiv struct {
	XMLName   xml.Name         `xml:"div"`
	Subtitles []TTMLInSubtitle `xml:"p"`

	Begin         *TTMLInDuration `xml:"begin,attr,omitempty"`
	Duration      *TTMLInDuration `xml:"dur,attr,omitempty"`
	End           *TTMLInDuration `xml:"end,attr,omitempty"`
	TimeContainer string          `xml:"timeContainer,attr,omitempty"`

	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}
type TTMLInBody struct {
	XMLName xml.Name        `xml:"body"`
	Divs    []TTMLInBodyDiv `xml:"div"`

	Begin         *TTMLInDuration `xml:"begin,attr,omitempty"`
	Duration      *TTMLInDuration `xml:"dur,attr,omitempty"`
	End           *TTMLInDuration `xml:"end,attr,omitempty"`
	TimeContainer string          `xml:"timeContainer,attr,omitempty"`

	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLIn represents an input TTML that must be unmarshaled
// We split it from the output TTML as we can't add strict namespace without breaking retrocompatibility
type TTMLIn struct {
	Framerate int            `xml:"frameRate,attr"`
	Lang      string         `xml:"lang,attr"`
	Metadata  TTMLInMetadata `xml:"head>metadata"`
	Regions   []TTMLInRegion `xml:"head>layout>region"`
	Styles    []TTMLInStyle  `xml:"head>styling>style"`
	Body      TTMLInBody     `xml:"body"`
	Tickrate  int            `xml:"tickRate,attr"`
	XMLName   xml.Name       `xml:"tt"`
}

// metadata returns the Metadata of the TTML
func (t TTMLIn) metadata() (m *Metadata) {
	m = &Metadata{
		Framerate:     t.Framerate,
		Title:         t.Metadata.Title,
		TTMLCopyright: t.Metadata.Copyright,
	}
	if v, ok := ttmlLanguageMapping.Get(astikit.StrPad(t.Lang, ' ', 2, astikit.PadCut)); ok {
		m.Language = v.(string)
	}
	return
}

// TTMLInMetadata represents an input TTML Metadata
type TTMLInMetadata struct {
	Copyright string `xml:"copyright"`
	Title     string `xml:"title"`
}

// TTMLInStyleAttributes represents input TTML style attributes
type TTMLInStyleAttributes struct {
	BackgroundColor *string `xml:"backgroundColor,attr,omitempty"`
	Color           *string `xml:"color,attr,omitempty"`
	Direction       *string `xml:"direction,attr,omitempty"`
	Display         *string `xml:"display,attr,omitempty"`
	DisplayAlign    *string `xml:"displayAlign,attr,omitempty"`
	Extent          *string `xml:"extent,attr,omitempty"`
	FontFamily      *string `xml:"fontFamily,attr,omitempty"`
	FontSize        *string `xml:"fontSize,attr,omitempty"`
	FontStyle       *string `xml:"fontStyle,attr,omitempty"`
	FontWeight      *string `xml:"fontWeight,attr,omitempty"`
	LineHeight      *string `xml:"lineHeight,attr,omitempty"`
	Opacity         *string `xml:"opacity,attr,omitempty"`
	Origin          *string `xml:"origin,attr,omitempty"`
	Overflow        *string `xml:"overflow,attr,omitempty"`
	Padding         *string `xml:"padding,attr,omitempty"`
	ShowBackground  *string `xml:"showBackground,attr,omitempty"`
	TextAlign       *string `xml:"textAlign,attr,omitempty"`
	TextDecoration  *string `xml:"textDecoration,attr,omitempty"`
	TextOutline     *string `xml:"textOutline,attr,omitempty"`
	UnicodeBidi     *string `xml:"unicodeBidi,attr,omitempty"`
	Visibility      *string `xml:"visibility,attr,omitempty"`
	WrapOption      *string `xml:"wrapOption,attr,omitempty"`
	WritingMode     *string `xml:"writingMode,attr,omitempty"`
	ZIndex          *int    `xml:"zIndex,attr,omitempty"`
}

// StyleAttributes converts TTMLInStyleAttributes into a StyleAttributes
func (s TTMLInStyleAttributes) styleAttributes() (o *StyleAttributes) {
	o = &StyleAttributes{
		TTMLDirection:      s.Direction,
		TTMLDisplay:        s.Display,
		TTMLDisplayAlign:   s.DisplayAlign,
		TTMLExtent:         s.Extent,
		TTMLFontFamily:     s.FontFamily,
		TTMLFontSize:       s.FontSize,
		TTMLFontStyle:      s.FontStyle,
		TTMLFontWeight:     s.FontWeight,
		TTMLLineHeight:     s.LineHeight,
		TTMLOpacity:        s.Opacity,
		TTMLOrigin:         s.Origin,
		TTMLOverflow:       s.Overflow,
		TTMLPadding:        s.Padding,
		TTMLShowBackground: s.ShowBackground,
		TTMLTextAlign:      s.TextAlign,
		TTMLTextDecoration: s.TextDecoration,
		TTMLTextOutline:    s.TextOutline,
		TTMLUnicodeBidi:    s.UnicodeBidi,
		TTMLVisibility:     s.Visibility,
		TTMLWrapOption:     s.WrapOption,
		TTMLWritingMode:    s.WritingMode,
		TTMLZIndex:         s.ZIndex,
	}
	// Parse colors if present
	if s.Color != nil {
		o.TTMLColor = newColorFromHTMLString(*s.Color)
	}
	if s.BackgroundColor != nil {
		o.TTMLBackgroundColor = newColorFromHTMLString(*s.BackgroundColor)
	}
	o.propagateTTMLAttributes()
	return
}

// TTMLInHeader represents an input TTML header
type TTMLInHeader struct {
	ID    string `xml:"id,attr,omitempty"`
	Style string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLInRegion represents an input TTML region
type TTMLInRegion struct {
	TTMLInHeader
	XMLName xml.Name `xml:"region"`
}

// TTMLInStyle represents an input TTML style
type TTMLInStyle struct {
	TTMLInHeader
	XMLName xml.Name `xml:"style"`
}

// TTMLInSubtitle represents an input TTML subtitle
type TTMLInSubtitle struct {
	Begin    *TTMLInDuration `xml:"begin,attr,omitempty"`
	Duration *TTMLInDuration `xml:"dur,attr,omitempty"`
	End      *TTMLInDuration `xml:"end,attr,omitempty"`
	ID       string          `xml:"id,attr,omitempty"`
	// We must store inner XML temporarily here since there's no tag to describe both any tag and chardata
	// Real unmarshal will be done manually afterwards
	Items  string `xml:",innerxml"`
	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLInItems represents input TTML items
type TTMLInItems []TTMLInItem

// UnmarshalXML implements the XML unmarshaler interface
func (i *TTMLInItems) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	// Get next tokens
	var t xml.Token
	for {
		// Get next token
		if t, err = d.Token(); err != nil {
			if err == io.EOF {
				break
			}
			err = fmt.Errorf("astisub: getting next token failed: %w", err)
			return
		}

		// Start element
		if se, ok := t.(xml.StartElement); ok {
			var e = TTMLInItem{}
			if err = d.DecodeElement(&e, &se); err != nil {
				err = fmt.Errorf("astisub: decoding xml.StartElement failed: %w", err)
				return
			}
			*i = append(*i, e)
		} else if b, ok := t.(xml.CharData); ok {
			if str := string(b); len(strings.TrimSpace(str)) > 0 {
				*i = append(*i, TTMLInItem{Text: str})
			}
		}
	}
	return nil
}

type ttmlXmlTokenReader struct {
	xmlTokenReader xml.TokenReader
	holdingToken   xml.Token
}

// Token implements the TokenReader interface, when it meets the "br" tag, it will hold the token and return a newline
// instead. This is to work around the fact that the go xml unmarshaler will ignore the "br" tag if it's within a
// character data field.
func (r *ttmlXmlTokenReader) Token() (xml.Token, error) {
	if r.holdingToken != nil {
		returnToken := r.holdingToken
		r.holdingToken = nil
		return returnToken, nil
	}

	t, err := r.xmlTokenReader.Token()
	if err != nil {
		return nil, err
	}

	if se, ok := t.(xml.StartElement); ok && strings.ToLower(se.Name.Local) == "br" {
		r.holdingToken = t
		return xml.CharData("\n"), nil
	}

	return t, nil
}

func newTTMLXmlDecoder(s string) *xml.Decoder {
	return xml.NewTokenDecoder(
		&ttmlXmlTokenReader{
			xmlTokenReader: xml.NewDecoder(strings.NewReader("<p>" + s + "</p>")),
			holdingToken:   nil,
		},
	)
}

// TTMLInItem represents an input TTML item
type TTMLInItem struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	TTMLInStyleAttributes
	XMLName xml.Name
}

// TTMLInDuration represents an input TTML duration
type TTMLInDuration struct {
	d                 time.Duration
	frames, framerate int // Framerate is in frame/s
	ticks, tickrate   int // Tickrate is in ticks/s
}

// UnmarshalText implements the TextUnmarshaler interface
// Possible formats are:
// - hh:mm:ss.mmm
// - hh:mm:ss:fff (fff being frames)
// - [ticks]t ([ticks] being the tick amount)
func (d *TTMLInDuration) UnmarshalText(i []byte) (err error) {
	// Reset duration
	d.d = time.Duration(0)
	d.frames = 0
	d.ticks = 0

	// Check offset time
	text := string(i)
	if matches := ttmlRegexpOffsetTime.FindStringSubmatch(text); matches != nil {
		// Parse value
		var value float64
		if value, err = strconv.ParseFloat(matches[1], 64); err != nil {
			err = fmt.Errorf("astisub: failed to parse value %s", matches[1])
			return
		}

		// Parse metric
		metric := matches[3]

		// Update duration
		if metric == "t" {
			d.ticks = int(value)
		} else if metric == "f" {
			d.frames = int(value)
		} else {
			// Get timebase
			var timebase time.Duration
			switch metric {
			case "h":
				timebase = time.Hour
			case "m":
				timebase = time.Minute
			case "s":
				timebase = time.Second
			case "ms":
				timebase = time.Millisecond
			default:
				err = fmt.Errorf("astisub: invalid metric %s", metric)
				return
			}

			// Update duration
			nanos := value * float64(timebase.Nanoseconds())
			// time.Duration is an int64 nanosecond count: reject values that
			// would overflow it instead of silently wrapping around.
			if nanos >= float64(math.MaxInt64) || nanos < float64(math.MinInt64) {
				err = fmt.Errorf("astisub: time overflow while parsing %q", text)
				return
			}
			d.d = time.Duration(nanos)
		}
		return
	}

	// Extract clock time frames
	if indexes := ttmlRegexpClockTimeFrames.FindStringIndex(text); indexes != nil {
		// Parse frames
		var s = text[indexes[0]+1 : indexes[1]]
		if d.frames, err = strconv.Atoi(s); err != nil {
			err = fmt.Errorf("astisub: atoi %s failed: %w", s, err)
			return
		}

		// Update text
		text = text[:indexes[0]] + ".000"
	}

	d.d, err = parseDuration(text, ".", 3)
	return
}

// duration returns the input TTML Duration's time.Duration
func (d TTMLInDuration) duration() (o time.Duration) {
	if d.ticks > 0 && d.tickrate > 0 {
		return time.Duration(float64(d.ticks) * 1e9 / float64(d.tickrate))
	}
	o = d.d
	if d.frames > 0 && d.framerate > 0 {
		o += time.Duration(float64(d.frames) / float64(d.framerate) * float64(time.Second.Nanoseconds()))
	}
	return
}

// ttmlSafeAddDuration adds two durations and returns an error if the result
// overflows time.Duration instead of wrapping around silently.
func ttmlSafeAddDuration(a, b time.Duration) (time.Duration, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, fmt.Errorf("astisub: time overflow adding %s and %s", a, b)
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, fmt.Errorf("astisub: time overflow adding %s and %s", a, b)
	}
	return a + b, nil
}

// ttmlIsSequentialContainer reports whether a timeContainer attribute value
// selects sequential containment ("seq"). Any other value, including the
// empty string, falls back to parallel ("par").
func ttmlIsSequentialContainer(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), ttmlTimeContainerSequential)
}

// ttmlResolvedTiming is the absolute timeline interval of a <p>.
type ttmlResolvedTiming struct {
	StartAt time.Duration
	EndAt   time.Duration
}

// ttmlContainerRange returns the declared absolute range [begin, end] of the
// parent container. A zero-value boolean means the bound is not declared.
func ttmlContainerRange(begin, end, dur *TTMLInDuration) (beginAt time.Duration, hasBegin bool, endAt time.Duration, hasEnd bool, err error) {
	if begin != nil {
		beginAt = begin.duration()
		hasBegin = true
	}
	if end != nil {
		endAt = end.duration()
		hasEnd = true
	} else if dur != nil {
		var d = dur.duration()
		if d < 0 {
			err = fmt.Errorf("astisub: container has negative duration %s", d)
			return
		}
		if endAt, err = ttmlSafeAddDuration(beginAt, d); err != nil {
			err = fmt.Errorf("astisub: container end overflows: %w", err)
			return
		}
		hasEnd = true
	}
	return
}

// resolveTTMLDivTimings computes the absolute timeline of every <p> of a <div>.
//
// With the default parallel containment ("par", also used when the attribute is
// absent), begin/end keep being interpreted on the document timeline and a <p>
// without timing inherits the <div>'s, preserving historical behaviour.
//
// With sequential containment ("seq", either set on the <div> or inherited from
// a <body timeContainer="seq">), each <p> is anchored to the previous <p>'s
// actual end (or the <div> begin for the first one). An explicit begin/end is
// an offset relative to that anchor. All arithmetic is overflow safe and
// malformed timings (missing end, negative duration, overflow or an interval
// escaping the declared parent range) produce an explicit error rather than
// overlapping or dropped subtitles.
func resolveTTMLDivTimings(body *TTMLInBody, div *TTMLInBodyDiv, framerate, tickrate int, inheritedAnchor *time.Duration) (timings []ttmlResolvedTiming, nextAnchor time.Duration, err error) {
	timings = make([]ttmlResolvedTiming, len(div.Subtitles))

	// Init frame/tick rates on container durations
	for _, d := range []*TTMLInDuration{div.Begin, div.End, div.Duration} {
		if d != nil {
			d.framerate = framerate
			d.tickrate = tickrate
		}
	}
	if body != nil {
		for _, d := range []*TTMLInDuration{body.Begin, body.End, body.Duration} {
			if d != nil {
				d.framerate = framerate
				d.tickrate = tickrate
			}
		}
	}

	// A <div> without an explicit timeContainer inherits a sequential <body>.
	var seq = ttmlIsSequentialContainer(div.TimeContainer)
	if strings.TrimSpace(div.TimeContainer) == "" && body != nil && ttmlIsSequentialContainer(body.TimeContainer) {
		seq = true
	}

	// The chain anchor starts at the container begin. When a sequential body
	// chains across several <div>s, the caller supplies the previous div end.
	var anchor time.Duration
	switch {
	case div.Begin != nil:
		anchor = div.Begin.duration()
	case inheritedAnchor != nil:
		anchor = *inheritedAnchor
	}
	nextAnchor = anchor

	// Declared parent range, if any. The <div>'s own bounds win over the
	// <body>'s; a missing bound simply isn't checked.
	var parentBegin, parentEnd time.Duration
	var hasParentBegin, hasParentEnd bool
	if parentBegin, hasParentBegin, parentEnd, hasParentEnd, err = ttmlContainerRange(div.Begin, div.End, div.Duration); err != nil {
		return
	}
	if body != nil {
		bodyBegin, bodyHasBegin, bodyEnd, bodyHasEnd, bodyErr := ttmlContainerRange(body.Begin, body.End, body.Duration)
		if bodyErr != nil {
			err = bodyErr
			return
		}
		if !hasParentBegin && bodyHasBegin {
			parentBegin, hasParentBegin = bodyBegin, true
		}
		if !hasParentEnd && bodyHasEnd {
			parentEnd, hasParentEnd = bodyEnd, true
		}
	}

	for i := range div.Subtitles {
		var ts = &div.Subtitles[i]

		// Init frame/tick rates
		for _, d := range []*TTMLInDuration{ts.Begin, ts.Duration, ts.End} {
			if d != nil {
				d.framerate = framerate
				d.tickrate = tickrate
			}
		}

		// Start and end
		var start, end time.Duration
		if !seq {
			// A <p> without its own timing takes the <div>'s. This historical
			// inheritance also drives the "no begin or end time" error.
			if ts.Begin == nil {
				ts.Begin = div.Begin
			}
			if ts.End == nil && ts.Duration == nil {
				ts.End = div.End
			}
			if ts.Begin == nil || (ts.End == nil && ts.Duration == nil) {
				err = fmt.Errorf("astisub: subtitle %q has no begin or end time", ts.ID)
				return
			}

			// Parallel containment: times live on the document timeline.
			start = ts.Begin.duration()
			if ts.End != nil {
				end = ts.End.duration()
			} else {
				var d = ts.Duration.duration()
				if d < 0 {
					err = fmt.Errorf("astisub: subtitle %q has negative duration %s", ts.ID, d)
					return
				}
				if end, err = ttmlSafeAddDuration(start, d); err != nil {
					err = fmt.Errorf("astisub: subtitle %q end: %w", ts.ID, err)
					return
				}
			}
		} else {
			// Sequential containment: begin (if any) is an offset relative to
			// the previous actual end, or to the container begin for the first.
			start = anchor
			if ts.Begin != nil {
				if start, err = ttmlSafeAddDuration(anchor, ts.Begin.duration()); err != nil {
					err = fmt.Errorf("astisub: subtitle %q begin: %w", ts.ID, err)
					return
				}
			}

			switch {
			case ts.End != nil:
				// The <p>'s own end is an offset relative to the same anchor.
				if end, err = ttmlSafeAddDuration(anchor, ts.End.duration()); err != nil {
					err = fmt.Errorf("astisub: subtitle %q end: %w", ts.ID, err)
					return
				}
			case ts.Duration != nil:
				var d = ts.Duration.duration()
				if d < 0 {
					err = fmt.Errorf("astisub: subtitle %q has negative duration %s", ts.ID, d)
					return
				}
				if end, err = ttmlSafeAddDuration(start, d); err != nil {
					err = fmt.Errorf("astisub: subtitle %q end: %w", ts.ID, err)
					return
				}
			case div.End != nil:
				// Inherited absolute container end.
				end = div.End.duration()
			case div.Duration != nil:
				// Absolute container end expressed as begin+dur.
				end = parentEnd
			default:
				err = fmt.Errorf("astisub: subtitle %q in sequential time container has no valid end time", ts.ID)
				return
			}
		}

		// A negative effective duration (end strictly before start) is invalid.
		if end < start {
			err = fmt.Errorf("astisub: subtitle %q has negative duration: start %s is after end %s", ts.ID, start, end)
			return
		}

		// The interval must stay inside the declared parent time container.
		if hasParentBegin && start < parentBegin {
			err = fmt.Errorf("astisub: subtitle %q starts at %s before parent begin %s", ts.ID, start, parentBegin)
			return
		}
		if hasParentEnd && end > parentEnd {
			err = fmt.Errorf("astisub: subtitle %q ends at %s after parent end %s", ts.ID, end, parentEnd)
			return
		}

		timings[i] = ttmlResolvedTiming{StartAt: start, EndAt: end}

		// Next sequential subtitle chains from this actual end.
		if seq {
			anchor = end
		}
		// Track the container's active end for body-wide chaining.
		if end > nextAnchor {
			nextAnchor = end
		}
	}
	return
}

// ReadFromTTML parses a .ttml content
func ReadFromTTML(i io.Reader) (o *Subtitles, err error) {
	// Init
	o = NewSubtitles()

	// Unmarshal XML
	var ttml TTMLIn
	if err = xml.NewDecoder(i).Decode(&ttml); err != nil {
		err = fmt.Errorf("astisub: xml decoding failed: %w", err)
		return
	}

	// Add metadata
	o.Metadata = ttml.metadata()

	// Loop through styles
	var parentStyles = make(map[string]*Style)
	for _, ts := range ttml.Styles {
		var s = &Style{
			ID:          ts.ID,
			InlineStyle: ts.TTMLInStyleAttributes.styleAttributes(),
		}
		o.Styles[s.ID] = s
		if len(ts.Style) > 0 {
			parentStyles[ts.Style] = s
		}
	}

	// Take care of parent styles
	for id, s := range parentStyles {
		if _, ok := o.Styles[id]; !ok {
			err = fmt.Errorf("astisub: Style %s requested by style %s doesn't exist", id, s.ID)
			return
		}
		s.Style = o.Styles[id]
	}

	// Loop through regions
	for _, tr := range ttml.Regions {
		var r = &Region{
			ID:          tr.ID,
			InlineStyle: tr.TTMLInStyleAttributes.styleAttributes(),
		}
		if len(tr.Style) > 0 {
			if _, ok := o.Styles[tr.Style]; !ok {
				err = fmt.Errorf("astisub: Style %s requested by region %s doesn't exist", tr.Style, r.ID)
				return
			}
			r.Style = o.Styles[tr.Style]
		}
		o.Regions[r.ID] = r
	}

	// Loop through subtitles
	bodyInlineStyle := ttml.Body.TTMLInStyleAttributes.styleAttributes()
	var bodySequential = ttmlIsSequentialContainer(ttml.Body.TimeContainer)
	var bodyAnchor time.Duration
	if ttml.Body.Begin != nil {
		bodyAnchor = ttml.Body.Begin.duration()
	}
	for _, div := range ttml.Body.Divs {
		divInlineStyle := div.TTMLInStyleAttributes.styleAttributes()

		// Propagate styles from Body -> Div
		divInlineStyle.merge(bodyInlineStyle)
		if div.Region == "" {
			div.Region = ttml.Body.Region
		}
		if div.Style == "" {
			div.Style = ttml.Body.Style
		}

		// A sequential body chains <div>s that do not declare their own begin.
		var inheritedAnchor *time.Duration
		if bodySequential && div.Begin == nil {
			inheritedAnchor = &bodyAnchor
		}

		// Resolve the timeline of every <p> up front. This implements parallel
		// (default) and sequential (timeContainer="seq") containment and makes
		// malformed timings fail before any item is appended.
		timings, nextAnchor, timingErr := resolveTTMLDivTimings(&ttml.Body, &div, ttml.Framerate, ttml.Tickrate, inheritedAnchor)
		if timingErr != nil {
			err = timingErr
			return
		}
		if bodySequential {
			bodyAnchor = nextAnchor
		}
		for i, ts := range div.Subtitles {
			itemInlineStyle := ts.TTMLInStyleAttributes.styleAttributes()

			// Propagate styles from Body -> Div -> Item.
			// If the item has its own Region, Style, or InlineStyle, it overrides the Div's.
			// This ensures all relevant styles are preserved at the item level,
			// maintaining compatibility with existing logic that relies on the Subtitles structure.
			itemInlineStyle.merge(divInlineStyle)
			if ts.Region == "" {
				ts.Region = div.Region
			}
			if ts.Style == "" {
				ts.Style = div.Style
			}

			var s = &Item{
				InlineStyle: itemInlineStyle,
				StartAt:     timings[i].StartAt,
				EndAt:       timings[i].EndAt,
			}

			// Add region
			if len(ts.Region) > 0 {
				if _, ok := o.Regions[ts.Region]; !ok {
					err = fmt.Errorf("astisub: Region %s requested by subtitle between %s and %s doesn't exist", ts.Region, s.StartAt, s.EndAt)
					return
				}
				s.Region = o.Regions[ts.Region]
			}

			// Add style
			if len(ts.Style) > 0 {
				if _, ok := o.Styles[ts.Style]; !ok {
					err = fmt.Errorf("astisub: Style %s requested by subtitle between %s and %s doesn't exist", ts.Style, s.StartAt, s.EndAt)
					return
				}
				s.Style = o.Styles[ts.Style]
			}

			// Remove items identation
			lines := strings.Split(ts.Items, "\n")
			for i := 0; i < len(lines); i++ {
				lines[i] = strings.TrimLeftFunc(lines[i], unicode.IsSpace)
			}

			// Unmarshal items
			var items = TTMLInItems{}
			if err = newTTMLXmlDecoder(strings.Join(lines, "")).Decode(&items); err != nil {
				err = fmt.Errorf("astisub: unmarshaling items failed: %w", err)
				return
			}

			// Loop through texts
			var l = &Line{}
			for _, tt := range items {
				// New line specified with the "br" tag
				if strings.ToLower(tt.XMLName.Local) == "br" {
					s.Lines = append(s.Lines, *l)
					l = &Line{}
					continue
				}

				// New line decoded as a line break. This can happen if there's a "br" tag within the text since
				// since the go xml unmarshaler will unmarshal a "br" tag as a line break if the field has the
				// chardata xml tag.
				for idx, li := range strings.Split(tt.Text, "\n") {
					// New line
					if idx > 0 {
						s.Lines = append(s.Lines, *l)
						l = &Line{}
					}

					// Init line item
					var t = LineItem{
						InlineStyle: tt.TTMLInStyleAttributes.styleAttributes(),
						Text:        li,
					}

					// Add style
					if len(tt.Style) > 0 {
						if _, ok := o.Styles[tt.Style]; !ok {
							err = fmt.Errorf("astisub: Style %s requested by item with text %s doesn't exist", tt.Style, tt.Text)
							return
						}
						t.Style = o.Styles[tt.Style]
					}

					// Append items
					l.Items = append(l.Items, t)
				}

			}
			s.Lines = append(s.Lines, *l)

			// Append subtitle
			o.Items = append(o.Items, s)
		}
	}
	return
}

// TTMLOut represents an output TTML that must be marshaled
// We split it from the input TTML as this time we'll add strict namespaces
type TTMLOut struct {
	Lang            string            `xml:"xml:lang,attr,omitempty"`
	Metadata        *TTMLOutMetadata  `xml:"head>metadata,omitempty"`
	Styles          []TTMLOutStyle    `xml:"head>styling>style,omitempty"` //!\\ Order is important! Keep Styling above Layout
	Regions         []TTMLOutRegion   `xml:"head>layout>region,omitempty"`
	Subtitles       []TTMLOutSubtitle `xml:"body>div>p,omitempty"`
	XMLName         xml.Name          `xml:"http://www.w3.org/ns/ttml tt"`
	XMLNamespaceTTM string            `xml:"xmlns:ttm,attr"`
	XMLNamespaceTTS string            `xml:"xmlns:tts,attr"`
}

// TTMLOutMetadata represents an output TTML Metadata
type TTMLOutMetadata struct {
	Copyright string `xml:"ttm:copyright,omitempty"`
	Title     string `xml:"ttm:title,omitempty"`
}

// TTMLOutStyleAttributes represents output TTML style attributes
type TTMLOutStyleAttributes struct {
	BackgroundColor *string `xml:"tts:backgroundColor,attr,omitempty"`
	Color           *string `xml:"tts:color,attr,omitempty"`
	Direction       *string `xml:"tts:direction,attr,omitempty"`
	Display         *string `xml:"tts:display,attr,omitempty"`
	DisplayAlign    *string `xml:"tts:displayAlign,attr,omitempty"`
	Extent          *string `xml:"tts:extent,attr,omitempty"`
	FontFamily      *string `xml:"tts:fontFamily,attr,omitempty"`
	FontSize        *string `xml:"tts:fontSize,attr,omitempty"`
	FontStyle       *string `xml:"tts:fontStyle,attr,omitempty"`
	FontWeight      *string `xml:"tts:fontWeight,attr,omitempty"`
	LineHeight      *string `xml:"tts:lineHeight,attr,omitempty"`
	Opacity         *string `xml:"tts:opacity,attr,omitempty"`
	Origin          *string `xml:"tts:origin,attr,omitempty"`
	Overflow        *string `xml:"tts:overflow,attr,omitempty"`
	Padding         *string `xml:"tts:padding,attr,omitempty"`
	ShowBackground  *string `xml:"tts:showBackground,attr,omitempty"`
	TextAlign       *string `xml:"tts:textAlign,attr,omitempty"`
	TextDecoration  *string `xml:"tts:textDecoration,attr,omitempty"`
	TextOutline     *string `xml:"tts:textOutline,attr,omitempty"`
	UnicodeBidi     *string `xml:"tts:unicodeBidi,attr,omitempty"`
	Visibility      *string `xml:"tts:visibility,attr,omitempty"`
	WrapOption      *string `xml:"tts:wrapOption,attr,omitempty"`
	WritingMode     *string `xml:"tts:writingMode,attr,omitempty"`
	ZIndex          *int    `xml:"tts:zIndex,attr,omitempty"`
}

// ttmlOutStyleAttributesFromStyleAttributes converts StyleAttributes into a TTMLOutStyleAttributes
func ttmlOutStyleAttributesFromStyleAttributes(s *StyleAttributes) TTMLOutStyleAttributes {
	if s == nil {
		return TTMLOutStyleAttributes{}
	}
	var color *string
	if s.TTMLColor != nil {
		// Try to convert to named color first, fall back to hex
		color = astikit.StrPtr(s.TTMLColor.HTMLString())
	}
	var backgroundColor *string
	if s.TTMLBackgroundColor != nil {
		backgroundColor = astikit.StrPtr(s.TTMLBackgroundColor.HTMLString())
	}
	return TTMLOutStyleAttributes{
		BackgroundColor: backgroundColor,
		Color:           color,
		Direction:       s.TTMLDirection,
		Display:         s.TTMLDisplay,
		DisplayAlign:    s.TTMLDisplayAlign,
		Extent:          s.TTMLExtent,
		FontFamily:      s.TTMLFontFamily,
		FontSize:        s.TTMLFontSize,
		FontStyle:       s.TTMLFontStyle,
		FontWeight:      s.TTMLFontWeight,
		LineHeight:      s.TTMLLineHeight,
		Opacity:         s.TTMLOpacity,
		Origin:          s.TTMLOrigin,
		Overflow:        s.TTMLOverflow,
		Padding:         s.TTMLPadding,
		ShowBackground:  s.TTMLShowBackground,
		TextAlign:       s.TTMLTextAlign,
		TextDecoration:  s.TTMLTextDecoration,
		TextOutline:     s.TTMLTextOutline,
		UnicodeBidi:     s.TTMLUnicodeBidi,
		Visibility:      s.TTMLVisibility,
		WrapOption:      s.TTMLWrapOption,
		WritingMode:     s.TTMLWritingMode,
		ZIndex:          s.TTMLZIndex,
	}
}

// TTMLOutHeader represents an output TTML header
type TTMLOutHeader struct {
	ID    string `xml:"xml:id,attr,omitempty"`
	Style string `xml:"style,attr,omitempty"`
	TTMLOutStyleAttributes
}

// TTMLOutRegion represents an output TTML region
type TTMLOutRegion struct {
	TTMLOutHeader
	XMLName xml.Name `xml:"region"`
}

// TTMLOutStyle represents an output TTML style
type TTMLOutStyle struct {
	TTMLOutHeader
	XMLName xml.Name `xml:"style"`
}

// TTMLOutSubtitle represents an output TTML subtitle
type TTMLOutSubtitle struct {
	Begin  TTMLOutDuration `xml:"begin,attr"`
	End    TTMLOutDuration `xml:"end,attr"`
	ID     string          `xml:"id,attr,omitempty"`
	Items  []TTMLOutItem
	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLOutStyleAttributes
}

// TTMLOutItem represents an output TTML Item
type TTMLOutItem struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	TTMLOutStyleAttributes
	XMLName xml.Name
}

// TTMLOutDuration represents an output TTML duration
type TTMLOutDuration time.Duration

// MarshalText implements the TextMarshaler interface
func (t TTMLOutDuration) MarshalText() ([]byte, error) {
	return []byte(formatDuration(time.Duration(t), ".", 3)), nil
}

// WriteToTTMLOptions represents TTML write options.
type WriteToTTMLOptions struct {
	Indent string // Default is 4 spaces.
}

// WriteToTTMLOption represents a WriteToTTML option.
type WriteToTTMLOption func(o *WriteToTTMLOptions)

// WriteToTTMLWithIndentOption sets the indent option.
func WriteToTTMLWithIndentOption(indent string) WriteToTTMLOption {
	return func(o *WriteToTTMLOptions) {
		o.Indent = indent
	}
}

// WriteToTTML writes subtitles in .ttml format
func (s Subtitles) WriteToTTML(o io.Writer, opts ...WriteToTTMLOption) (err error) {
	// Create write options
	wo := &WriteToTTMLOptions{Indent: "    "}
	for _, opt := range opts {
		opt(wo)
	}

	// Do not write anything if no subtitles
	if len(s.Items) == 0 {
		return ErrNoSubtitlesToWrite
	}

	// Init TTML
	var ttml = TTMLOut{
		XMLNamespaceTTM: "http://www.w3.org/ns/ttml#metadata",
		XMLNamespaceTTS: "http://www.w3.org/ns/ttml#styling",
	}

	// Add metadata
	if s.Metadata != nil {
		if v, ok := ttmlLanguageMapping.GetInverse(s.Metadata.Language); ok {
			ttml.Lang = v.(string)
		}
		if len(s.Metadata.TTMLCopyright) > 0 || len(s.Metadata.Title) > 0 {
			ttml.Metadata = &TTMLOutMetadata{
				Copyright: s.Metadata.TTMLCopyright,
				Title:     s.Metadata.Title,
			}
		}
	}

	// Add regions
	var k []string
	for _, region := range s.Regions {
		k = append(k, region.ID)
	}
	sort.Strings(k)
	for _, id := range k {
		var ttmlRegion = TTMLOutRegion{TTMLOutHeader: TTMLOutHeader{
			ID:                     s.Regions[id].ID,
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(s.Regions[id].InlineStyle),
		}}
		if s.Regions[id].Style != nil {
			ttmlRegion.Style = s.Regions[id].Style.ID
		}
		ttml.Regions = append(ttml.Regions, ttmlRegion)
	}

	// Add styles
	k = []string{}
	for _, style := range s.Styles {
		k = append(k, style.ID)
	}
	sort.Strings(k)
	for _, id := range k {
		var ttmlStyle = TTMLOutStyle{TTMLOutHeader: TTMLOutHeader{
			ID:                     s.Styles[id].ID,
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(s.Styles[id].InlineStyle),
		}}
		if s.Styles[id].Style != nil {
			ttmlStyle.Style = s.Styles[id].Style.ID
		}
		ttml.Styles = append(ttml.Styles, ttmlStyle)
	}

	// Add items
	for _, item := range s.Items {
		// Init subtitle
		var ttmlSubtitle = TTMLOutSubtitle{
			Begin:                  TTMLOutDuration(item.StartAt),
			End:                    TTMLOutDuration(item.EndAt),
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(item.InlineStyle),
		}

		// Add region
		if item.Region != nil {
			ttmlSubtitle.Region = item.Region.ID
		}

		// Add style
		if item.Style != nil {
			ttmlSubtitle.Style = item.Style.ID
		}

		// Add lines
		for _, line := range item.Lines {
			// Loop through line items
			for _, lineItem := range line.Items {
				// Init ttml item
				var ttmlItem = TTMLOutItem{
					Text:                   lineItem.Text,
					TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(lineItem.InlineStyle),
					XMLName:                xml.Name{Local: "span"},
				}

				// Add style
				if lineItem.Style != nil {
					ttmlItem.Style = lineItem.Style.ID
				}

				// Add ttml item
				ttmlSubtitle.Items = append(ttmlSubtitle.Items, ttmlItem)
			}

			// Add line break
			ttmlSubtitle.Items = append(ttmlSubtitle.Items, TTMLOutItem{XMLName: xml.Name{Local: "br"}})
		}

		// Remove last line break
		if len(ttmlSubtitle.Items) > 0 {
			ttmlSubtitle.Items = ttmlSubtitle.Items[:len(ttmlSubtitle.Items)-1]
		}

		// Append subtitle
		ttml.Subtitles = append(ttml.Subtitles, ttmlSubtitle)
	}

	// Marshal XML
	var e = xml.NewEncoder(o)

	// Set indent
	e.Indent("", wo.Indent)

	if err = e.Encode(ttml); err != nil {
		err = fmt.Errorf("astisub: xml encoding failed: %w", err)
		return
	}
	return
}
