package astisub_test

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
)

func assertSSAStyle(t *testing.T, e, a astisub.Style) {
	assert.Equal(t, e.ID, a.ID)
	assertSSAStyleAttributes(t, *e.InlineStyle, *a.InlineStyle)
}

func assertSSAStyleAttributes(t *testing.T, e, a astisub.StyleAttributes) {
	if e.SSAAlignment != nil {
		assert.Equal(t, *e.SSAAlignment, *a.SSAAlignment)
	}
	if e.SSAAlphaLevel != nil {
		assert.Equal(t, *e.SSAAlphaLevel, *a.SSAAlphaLevel)
	}
	if e.SSABackColour != nil {
		assert.Equal(t, *e.SSABackColour, *a.SSABackColour)
	}
	if e.SSABold != nil {
		assert.Equal(t, *e.SSABold, *a.SSABold)
	}
	if e.SSABorderStyle != nil {
		assert.Equal(t, *e.SSABorderStyle, *a.SSABorderStyle)
	}
	if e.SSAFontSize != nil {
		assert.Equal(t, e.SSAFontName, a.SSAFontName)
	}
	if e.SSAFontSize != nil {
		assert.Equal(t, *e.SSAFontSize, *a.SSAFontSize)
	}
	if e.SSALayer != nil {
		assert.Equal(t, *e.SSALayer, *a.SSALayer)
	}
	if e.SSAMarked != nil {
		assert.Equal(t, *e.SSAMarked, *a.SSAMarked)
	}
	if e.SSAMarginLeft != nil {
		assert.Equal(t, *e.SSAMarginLeft, *a.SSAMarginLeft)
	}
	if e.SSAMarginRight != nil {
		assert.Equal(t, *e.SSAMarginRight, *a.SSAMarginRight)
	}
	if e.SSAMarginVertical != nil {
		assert.Equal(t, *e.SSAMarginVertical, *a.SSAMarginVertical)
	}
	if e.SSAOutline != nil {
		assert.Equal(t, *e.SSAOutline, *a.SSAOutline)
	}
	if e.SSAOutlineColour != nil {
		assert.Equal(t, *e.SSAOutlineColour, *a.SSAOutlineColour)
	}
	if e.SSAPrimaryColour != nil {
		assert.Equal(t, *e.SSAPrimaryColour, *a.SSAPrimaryColour)
	}
	if e.SSASecondaryColour != nil {
		assert.Equal(t, *e.SSASecondaryColour, *a.SSASecondaryColour)
	}
	if e.SSAShadow != nil {
		assert.Equal(t, *e.SSAShadow, *a.SSAShadow)
	}
}

func TestSSA(t *testing.T) {
	// Open
	s, err := astisub.OpenFile("./testdata/example-in.ssa")
	assert.NoError(t, err)
	assertSubtitleItems(t, s)
	// Metadata
	assert.Equal(t, &astisub.Metadata{Comments: []string{"Comment 1", "Comment 2"}, SSACollisions: "Normal", SSAOriginalScript: "asticode", SSAPlayDepth: astikit.IntPtr(0), SSAPlayResY: astikit.IntPtr(600), SSAScriptType: "v4.00", SSAScriptUpdatedBy: "version 2.8.01", SSATimer: astikit.Float64Ptr(100), Title: "SSA test"}, s.Metadata)
	// Styles
	assert.Equal(t, 3, len(s.Styles))
	assertSSAStyle(t, astisub.Style{ID: "1", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(7), SSAAlphaLevel: astikit.Float64Ptr(0.1), SSABackColour: &astisub.Color{Alpha: 128, Red: 8}, SSABold: astikit.BoolPtr(true), SSABorderStyle: astikit.IntPtr(7), SSAFontName: "f1", SSAFontSize: astikit.Float64Ptr(4), SSAOutline: astikit.Float64Ptr(1), SSAOutlineColour: &astisub.Color{Green: 255, Red: 255}, SSAMarginLeft: astikit.IntPtr(1), SSAMarginRight: astikit.IntPtr(4), SSAMarginVertical: astikit.IntPtr(7), SSAPrimaryColour: &astisub.Color{Green: 255, Red: 255}, SSASecondaryColour: &astisub.Color{Green: 255, Red: 255}, SSAShadow: astikit.Float64Ptr(4)}}, *s.Styles["1"])
	assertSSAStyle(t, astisub.Style{ID: "2", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(8), SSAAlphaLevel: astikit.Float64Ptr(0.2), SSABackColour: &astisub.Color{Blue: 15, Green: 15, Red: 15}, SSABold: astikit.BoolPtr(true), SSABorderStyle: astikit.IntPtr(8), SSAEncoding: astikit.IntPtr(1), SSAFontName: "f2", SSAFontSize: astikit.Float64Ptr(5), SSAOutline: astikit.Float64Ptr(2), SSAOutlineColour: &astisub.Color{Green: 255, Red: 255}, SSAMarginLeft: astikit.IntPtr(2), SSAMarginRight: astikit.IntPtr(5), SSAMarginVertical: astikit.IntPtr(8), SSAPrimaryColour: &astisub.Color{Blue: 239, Green: 239, Red: 239}, SSASecondaryColour: &astisub.Color{Green: 255, Red: 255}, SSAShadow: astikit.Float64Ptr(5)}}, *s.Styles["2"])
	assertSSAStyle(t, astisub.Style{ID: "3", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(9), SSAAlphaLevel: astikit.Float64Ptr(0.3), SSABackColour: &astisub.Color{Red: 8}, SSABorderStyle: astikit.IntPtr(9), SSAEncoding: astikit.IntPtr(2), SSAFontName: "f3", SSAFontSize: astikit.Float64Ptr(6), SSAOutline: astikit.Float64Ptr(3), SSAOutlineColour: &astisub.Color{Red: 8}, SSAMarginLeft: astikit.IntPtr(3), SSAMarginRight: astikit.IntPtr(6), SSAMarginVertical: astikit.IntPtr(9), SSAPrimaryColour: &astisub.Color{Blue: 180, Green: 252, Red: 252}, SSASecondaryColour: &astisub.Color{Blue: 180, Green: 252, Red: 252}, SSAShadow: astikit.Float64Ptr(6)}}, *s.Styles["3"])
	// Items
	assertSSAStyleAttributes(t, astisub.StyleAttributes{SSAEffect: "test", SSAMarked: astikit.BoolPtr(false), SSAMarginLeft: astikit.IntPtr(1234), SSAMarginRight: astikit.IntPtr(2345), SSAMarginVertical: astikit.IntPtr(3456)}, *s.Items[0].InlineStyle)
	assert.Equal(t, s.Styles["1"], s.Items[0].Style)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{InlineStyle: &astisub.StyleAttributes{SSAEffect: "{\\pos(400,570)}"}, Text: "(deep rumbling)"}}, VoiceName: "Cher"}}, s.Items[0].Lines)
	assert.Equal(t, s.Styles["2"], s.Items[1].Style)
	assert.Equal(t, s.Styles["3"], s.Items[2].Style)
	assert.Equal(t, s.Styles["1"], s.Items[3].Style)
	assert.Equal(t, s.Styles["2"], s.Items[4].Style)
	assert.Equal(t, s.Styles["3"], s.Items[5].Style)

	// No subtitles to write
	w := &bytes.Buffer{}
	err = astisub.Subtitles{}.WriteToSSA(w)
	assert.EqualError(t, err, astisub.ErrNoSubtitlesToWrite.Error())

	// Write
	c, err := ioutil.ReadFile("./testdata/example-out.ssa")
	assert.NoError(t, err)
	err = s.WriteToSSA(w)
	assert.NoError(t, err)
	assert.Equal(t, string(c), w.String())
}

func TestSSAv4plus(t *testing.T) {
	// Open
	s, err := astisub.OpenFile("./testdata/example-in.ssa")
	assert.NoError(t, err)
	assertSubtitleItems(t, s)
	// Metadata
	s.Metadata.SSAScriptType = "v4.00+"

	// Write
	w := &bytes.Buffer{}
	err = s.WriteToSSA(w)
	assert.NoError(t, err)

	c, err := ioutil.ReadFile("./testdata/example-out-v4plus.ssa")
	assert.NoError(t, err)
	assert.Equal(t, string(c), w.String())
}

func TestWriteToSSANilMetadata(t *testing.T) {
	// Metadata intentionally left nil
	s := &astisub.Subtitles{
		Items: []*astisub.Item{
			{
				StartAt: time.Second,
				EndAt:   3 * time.Second,
				Lines:   []astisub.Line{{Items: []astisub.LineItem{{Text: "Hello world"}}}},
			},
		},
	}

	w := &bytes.Buffer{}
	assert.NotPanics(t, func() {
		s.WriteToSSA(w)
	})
}

func TestInBetweenSSAEffect(t *testing.T) {
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: Marked=0,0:01:39.00,0:01:41.04,,Cher,1234,2345,3456,test,First item{\pos(400,570)}Second item`)))
	assert.NoError(t, err)
	assert.Len(t, s.Items[0].Lines[0].Items, 2)
	assert.Equal(t, astisub.LineItem{Text: "First item"}, s.Items[0].Lines[0].Items[0])
	assert.Equal(t, astisub.LineItem{
		InlineStyle: &astisub.StyleAttributes{SSAEffect: "{\\pos(400,570)}"},
		Text:        "Second item",
	}, s.Items[0].Lines[0].Items[1])
}

func TestWriteToSSAKeepsInlineStyleSpacing(t *testing.T) {
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:03.50,Default,,0,0,0,,Hello {\i1}world{\i0}!`)))
	assert.NoError(t, err)
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `,Hello {\i1}world{\i0}!`)
}

// ssaLineText rebuilds the event text of a single-line item
func ssaLineText(i *astisub.Item) string {
	var sb strings.Builder
	for _, item := range i.Lines[0].Items {
		if item.InlineStyle != nil {
			sb.WriteString(item.InlineStyle.SSAEffect)
		}
		sb.WriteString(item.Text)
	}
	return sb.String()
}

func TestSSATransform(t *testing.T) {
	// Open the fixture holding two animated events
	s, err := astisub.OpenFile("./testdata/example-in-transform.ssa")
	assert.NoError(t, err)
	assert.Len(t, s.Items, 2)

	// The first event carries two transforms, their order is preserved
	parts, err := astisub.ParseSSAText(ssaLineText(s.Items[0]))
	assert.NoError(t, err)
	transforms := astisub.SSATransforms(parts)
	assert.Len(t, transforms, 2)

	// The color transform fades to red over the first second
	assert.True(t, transforms[0].HasTime)
	assert.Equal(t, time.Duration(0), transforms[0].Start)
	assert.Equal(t, time.Second, transforms[0].End)
	assert.Equal(t, 1.0, transforms[0].Acceleration)
	assert.Len(t, transforms[0].Tags, 1)
	assert.Equal(t, "1c", transforms[0].Tags[0].Name)
	assert.Equal(t, &astisub.Color{Red: 255}, transforms[0].Tags[0].Color())

	// The scale transform grows the text over the second one
	assert.Equal(t, time.Second, transforms[1].Start)
	assert.Equal(t, 2*time.Second, transforms[1].End)
	assert.Equal(t, 0.5, transforms[1].Acceleration)
	assert.Len(t, transforms[1].Tags, 2)
	x, ok := transforms[1].Tags[0].Scale()
	assert.True(t, ok)
	assert.Equal(t, 150.0, x)
	y, ok := transforms[1].Tags[1].Scale()
	assert.True(t, ok)
	assert.Equal(t, 150.0, y)

	// Writing the unmodified parts back reproduces the event text
	assert.Equal(t, ssaLineText(s.Items[0]), astisub.WriteSSAText(parts))

	// The second event mixes text, a transform and a plain override tag
	parts2, err := astisub.ParseSSAText(ssaLineText(s.Items[1]))
	assert.NoError(t, err)
	assert.Equal(t, ssaLineText(s.Items[1]), astisub.WriteSSAText(parts2))
	transforms2 := astisub.SSATransforms(parts2)
	assert.Len(t, transforms2, 1)
	assert.True(t, transforms2[0].HasTime)
	assert.Equal(t, 500*time.Millisecond, transforms2[0].Start)
	assert.Equal(t, 1500*time.Millisecond, transforms2[0].End)
	assert.Equal(t, 0.0, transforms2[0].Acceleration)

	// Adjust the animation window, the acceleration and the color target
	transforms[0].End = 2 * time.Second
	transforms[0].Tags[0] = astisub.NewSSAColorTransformTag("1c", &astisub.Color{Green: 255})
	transforms[1].Acceleration = 2
	s.Items[0].Lines[0].Items = []astisub.LineItem{{Text: astisub.WriteSSAText(parts)}}

	// The event timing is untouched
	assert.Equal(t, time.Second, s.Items[0].StartAt)
	assert.Equal(t, 5*time.Second, s.Items[0].EndAt)

	// Write
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `{\an8}{\t(0,2000,1,\1c&H00FF00&)}{\t(1000,2000,2,\fscx150\fscy150)}Fading red and growing`)
	assert.Contains(t, w.String(), `Shrinking {\t(500,1500,\fscx50)}text{\i1} stays`)
	assert.Contains(t, w.String(), `00:00:01.00,00:00:05.00`)

	// Re-read: the modified transforms survive the round-trip
	s2, err := astisub.ReadFromSSA(bytes.NewReader(w.Bytes()))
	assert.NoError(t, err)
	assert.Len(t, s2.Items, 2)
	parts3, err := astisub.ParseSSAText(ssaLineText(s2.Items[0]))
	assert.NoError(t, err)
	transforms3 := astisub.SSATransforms(parts3)
	assert.Len(t, transforms3, 2)
	assert.Equal(t, 2*time.Second, transforms3[0].End)
	assert.Equal(t, &astisub.Color{Green: 255}, transforms3[0].Tags[0].Color())
	assert.Equal(t, 2.0, transforms3[1].Acceleration)
}
