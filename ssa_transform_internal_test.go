package astisub

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseSSAText(t *testing.T) {
	for _, v := range []struct {
		name     string
		text     string
		expected []SSATextPart
	}{
		{
			name:     "no override",
			text:     "Hello world",
			expected: []SSATextPart{{Text: "Hello world"}},
		},
		{
			name: "plain override blocks are kept verbatim",
			text: `Hello {\i1 this is a comment}world{\an8}!`,
			expected: []SSATextPart{
				{Text: "Hello "},
				{Text: `{\i1 this is a comment}`},
				{Text: "world"},
				{Text: `{\an8}`},
				{Text: "!"},
			},
		},
		{
			name: "transform with time, acceleration and color",
			text: `{\t(0,1000,1,\1c&H0000FF&)}`,
			expected: []SSATextPart{{Transform: &SSATransform{
				Start:        0,
				End:          time.Second,
				HasTime:      true,
				Acceleration: 1,
				Tags:         []SSATransformTag{{Name: "1c", Args: "&H0000FF&"}},
			}}},
		},
		{
			name: "transform with time and scales",
			text: `{\t(500,1500,\fscx150\fscy50)}`,
			expected: []SSATextPart{{Transform: &SSATransform{
				Start:   500 * time.Millisecond,
				End:     1500 * time.Millisecond,
				HasTime: true,
				Tags: []SSATransformTag{
					{Name: "fscx", Args: "150"},
					{Name: "fscy", Args: "50"},
				},
			}}},
		},
		{
			name: "transform with acceleration only",
			text: `{\t(0.5,\fscx150)}`,
			expected: []SSATextPart{{Transform: &SSATransform{
				Acceleration: 0.5,
				Tags:         []SSATransformTag{{Name: "fscx", Args: "150"}},
			}}},
		},
		{
			name: "transform with tags only",
			text: `{\t(\c&HFFFFFF&)}`,
			expected: []SSATextPart{{Transform: &SSATransform{
				Tags: []SSATransformTag{{Name: "c", Args: "&HFFFFFF&"}},
			}}},
		},
		{
			name: "several transforms keep their order",
			text: `{\t(0,100,\fscx100)\t(100,200,\fscx200)}`,
			expected: []SSATextPart{
				{Transform: &SSATransform{
					Start:   0,
					End:     100 * time.Millisecond,
					HasTime: true,
					Tags:    []SSATransformTag{{Name: "fscx", Args: "100"}},
				}},
				{Transform: &SSATransform{
					Start:   100 * time.Millisecond,
					End:     200 * time.Millisecond,
					HasTime: true,
					Tags:    []SSATransformTag{{Name: "fscx", Args: "200"}},
				}},
			},
		},
		{
			name: "transforms mixed with plain tags in one block",
			text: `{\b1\t(0,100,\fscx150)\i1}`,
			expected: []SSATextPart{
				{Text: `{\b1}`},
				{Transform: &SSATransform{
					Start:   0,
					End:     100 * time.Millisecond,
					HasTime: true,
					Tags:    []SSATransformTag{{Name: "fscx", Args: "150"}},
				}},
				{Text: `{\i1}`},
			},
		},
		{
			name:     "unterminated brace is text",
			text:     `Hello {\t(0,100,\fscx150) world`,
			expected: []SSATextPart{{Text: `Hello {\t(0,100,\fscx150) world`}},
		},
	} {
		t.Run(v.name, func(t *testing.T) {
			parts, err := ParseSSAText(v.text)
			assert.NoError(t, err)
			assert.Equal(t, v.expected, parts)
		})
	}
}

func TestParseSSATextErrors(t *testing.T) {
	for _, v := range []struct {
		name string
		text string
		err  error
	}{
		{name: "missing parentheses", text: `{\t}`, err: ErrSSATransformMalformed},
		{name: "unbalanced parentheses", text: `{\t(0,500,\fscx150}`, err: ErrSSATransformMalformed},
		{name: "empty transform", text: `{\t()}`, err: ErrSSATransformMalformed},
		{name: "no target tag", text: `{\t(0,500)}`, err: ErrSSATransformMalformed},
		{name: "too many numeric arguments", text: `{\t(0,500,1,2,\fscx150)}`, err: ErrSSATransformMalformed},
		{name: "invalid color", text: `{\t(0,500,\1c&HZZ&)}`, err: ErrSSATransformMalformed},
		{name: "invalid scale", text: `{\t(0,500,\fscx1.2.3)}`, err: ErrSSATransformMalformed},
		{name: "negative time", text: `{\t(-1,500,\fscx150)}`, err: ErrSSATransformInvalidTime},
		{name: "end before start", text: `{\t(500,100,\fscx150)}`, err: ErrSSATransformInvalidTime},
		{name: "zero acceleration", text: `{\t(0,500,0,\fscx150)}`, err: ErrSSATransformInvalidAcceleration},
		{name: "negative acceleration", text: `{\t(-0.5,\fscx150)}`, err: ErrSSATransformInvalidAcceleration},
		{name: "unsupported tag", text: `{\t(0,500,\fs24)}`, err: ErrSSATransformUnsupportedTag},
		{name: "unsupported parenthesized tag", text: `{\t(0,500,\clip(0,0,1,1))}`, err: ErrSSATransformUnsupportedTag},
		{name: "nested transform", text: `{\t(0,500,\t(1,2,\fscx150))}`, err: ErrSSATransformUnsupportedTag},
	} {
		t.Run(v.name, func(t *testing.T) {
			parts, err := ParseSSAText(v.text)
			assert.True(t, errors.Is(err, v.err), "expected %v, got %v", v.err, err)
			// Nothing is swallowed: the text is kept verbatim
			assert.Equal(t, v.text, WriteSSAText(parts))
		})
	}
}

func TestWriteSSAText(t *testing.T) {
	// Unmodified parts round-trip
	for _, text := range []string{
		"Hello world",
		`{\an8}Hello {\i1}world{\i0}!`,
		`{\an8}{\t(0,1000,1,\1c&H0000FF&)}{\t(1000,2000,0.5,\fscx150\fscy150)}Fading red and growing`,
		`Shrinking {\t(500,1500,\fscx50)}text{\i1} stays`,
		`{\t(\c&HFFFFFF&)}`,
		`{\t(0.5,\fscx150)}`,
	} {
		parts, err := ParseSSAText(text)
		assert.NoError(t, err)
		assert.Equal(t, text, WriteSSAText(parts))
	}

	// A transform mixed with other tags is moved to its own block, tags and order preserved
	parts, err := ParseSSAText(`{\b1\t(0,100,\fscx150)\i1}`)
	assert.NoError(t, err)
	assert.Equal(t, `{\b1}{\t(0,100,\fscx150)}{\i1}`, WriteSSAText(parts))

	// Modified transforms are written back as valid ASS
	transforms := SSATransforms(parts)
	transforms[0].End = 2 * time.Second
	transforms[0].Acceleration = 2
	transforms[0].Tags = []SSATransformTag{
		NewSSAColorTransformTag("1c", &Color{Red: 255}),
		NewSSAScaleTransformTag("fscx", 150),
	}
	assert.Equal(t, `{\b1}{\t(0,2000,2,\1c&H0000FF&\fscx150)}{\i1}`, WriteSSAText(parts))
}

func TestSSATransformTag(t *testing.T) {
	// Color
	var tag = SSATransformTag{Name: "1c", Args: "&H0000FF&"}
	assert.Equal(t, &Color{Red: 255}, tag.Color())
	_, ok := tag.Scale()
	assert.False(t, ok)
	assert.Equal(t, tag, NewSSAColorTransformTag("1c", &Color{Red: 255}))

	// Scale
	tag = SSATransformTag{Name: "fscx", Args: "150"}
	assert.Nil(t, tag.Color())
	v, ok := tag.Scale()
	assert.True(t, ok)
	assert.Equal(t, 150.0, v)
	assert.Equal(t, tag, NewSSAScaleTransformTag("fscx", 150))

	// Neither
	tag = SSATransformTag{Name: "fs", Args: "24"}
	assert.Nil(t, tag.Color())
	_, ok = tag.Scale()
	assert.False(t, ok)
}
