package astisub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fadeTags(block string) []ssaOverrideTag {
	return splitSSAOverrides(block)[0].tags
}

func TestParseSSAFadeTag(t *testing.T) {
	for _, v := range []struct {
		expected *SSAFade
		name     string
		tag      ssaOverrideTag
	}{
		{
			name: "fad",
			tag:  ssaOverrideTag{name: "fad", args: "(100,200)"},
			expected: &SSAFade{
				Type:    SSAFadeTypeFAD,
				FadeIn:  100,
				FadeOut: 200,
			},
		},
		{
			name: "fad without parens",
			tag:  ssaOverrideTag{name: "fad", args: "100,200"},
			expected: &SSAFade{
				Type:    SSAFadeTypeFAD,
				FadeIn:  100,
				FadeOut: 200,
			},
		},
		{
			name: "fad with spaces",
			tag:  ssaOverrideTag{name: "fad", args: "( 100 , 200 )"},
			expected: &SSAFade{
				Type:    SSAFadeTypeFAD,
				FadeIn:  100,
				FadeOut: 200,
			},
		},
		{
			name: "fade",
			tag:  ssaOverrideTag{name: "fade", args: "(255,0,255,0,100,800,1000)"},
			expected: &SSAFade{
				Type:   SSAFadeTypeFADE,
				Alpha1: 255,
				Alpha2: 0,
				Alpha3: 255,
				Time1:  0,
				Time2:  100,
				Time3:  800,
				Time4:  1000,
			},
		},
		{
			name: "fade with custom alpha levels",
			tag:  ssaOverrideTag{name: "fade", args: "(128,16,200,100,200,700,900)"},
			expected: &SSAFade{
				Type:   SSAFadeTypeFADE,
				Alpha1: 128,
				Alpha2: 16,
				Alpha3: 200,
				Time1:  100,
				Time2:  200,
				Time3:  700,
				Time4:  900,
			},
		},
	} {
		t.Run(v.name, func(t *testing.T) {
			f, err := parseSSAFadeTag(v.tag)
			require.NoError(t, err)
			assert.Equal(t, v.expected, f)
		})
	}

	// Invalid tags all fail with the same sentinel and must be preserved as text
	for _, v := range []struct {
		name string
		tag  ssaOverrideTag
	}{
		{name: "fad missing an argument", tag: ssaOverrideTag{name: "fad", args: "(100)"}},
		{name: "fad too many arguments", tag: ssaOverrideTag{name: "fad", args: "(100,200,300)"}},
		{name: "fad non numeric", tag: ssaOverrideTag{name: "fad", args: "(foo,200)"}},
		{name: "fad negative", tag: ssaOverrideTag{name: "fad", args: "(-1,200)"}},
		{name: "fade missing arguments", tag: ssaOverrideTag{name: "fade", args: "(255,0,255,0,100,800)"}},
		{name: "fade too many arguments", tag: ssaOverrideTag{name: "fade", args: "(255,0,255,0,100,800,1000,1)"}},
		{name: "fade alpha above 255", tag: ssaOverrideTag{name: "fade", args: "(256,0,255,0,100,800,1000)"}},
		{name: "fade negative alpha", tag: ssaOverrideTag{name: "fade", args: "(255,-1,255,0,100,800,1000)"}},
		{name: "fade unordered times", tag: ssaOverrideTag{name: "fade", args: "(255,0,255,0,800,100,1000)"}},
		{name: "fade negative time", tag: ssaOverrideTag{name: "fade", args: "(255,0,255,-1,100,800,1000)"}},
		{name: "fade non numeric", tag: ssaOverrideTag{name: "fade", args: "(255,0,255,0,100,800,bar)"}},
		{name: "unrelated tag", tag: ssaOverrideTag{name: "i", args: "1"}},
	} {
		t.Run("invalid: "+v.name, func(t *testing.T) {
			_, err := parseSSAFadeTag(v.tag)
			assert.Equal(t, ErrInvalidSSAFade, err)
		})
	}
}

func TestSSAFadeTagRoundTrip(t *testing.T) {
	for _, block := range []string{
		`{\fad(100,200)}`,
		`{\fade(255,0,255,0,100,800,1000)}`,
		`{\fade(128,16,200,100,200,700,900)}`,
	} {
		tags := fadeTags(block)
		require.Len(t, tags, 1)
		f, err := parseSSAFadeTag(tags[0])
		require.NoError(t, err)
		rendered, err := f.overrideTag()
		require.NoError(t, err)
		assert.Equal(t, tags[0], rendered)
		assert.Equal(t, "{"+"\\"+rendered.name+rendered.args+"}", block)
	}
}

func TestSSAFadeNormalize(t *testing.T) {
	// \fad(100,200) on a one-second event is the canonical
	// \fade(255,0,255,0,100,800,1000)
	fad, err := parseSSAFadeTag(ssaOverrideTag{name: "fad", args: "(100,200)"})
	require.NoError(t, err)
	a1, a2, a3, t1, t2, t3, t4, err := fad.normalize(time.Second)
	require.NoError(t, err)
	assert.Equal(t, uint8(255), a1)
	assert.Equal(t, uint8(0), a2)
	assert.Equal(t, uint8(255), a3)
	assert.Equal(t, 0, t1)
	assert.Equal(t, 100, t2)
	assert.Equal(t, 800, t3)
	assert.Equal(t, 1000, t4)

	// \fade keeps its own boundary values
	fade, err := parseSSAFadeTag(ssaOverrideTag{name: "fade", args: "(128,16,200,100,200,700,900)"})
	require.NoError(t, err)
	a1, a2, a3, t1, t2, t3, t4, err = fade.normalize(time.Second)
	require.NoError(t, err)
	assert.Equal(t, uint8(128), a1)
	assert.Equal(t, uint8(16), a2)
	assert.Equal(t, uint8(200), a3)
	assert.Equal(t, 100, t1)
	assert.Equal(t, 200, t2)
	assert.Equal(t, 700, t3)
	assert.Equal(t, 900, t4)
}

// TestSSAFadeAlphaAtFAD checks the requirement's exact sample: a one-second
// line with \fad(100,200)
func TestSSAFadeAlphaAtFAD(t *testing.T) {
	f, err := parseSSAFadeTag(ssaOverrideTag{name: "fad", args: "(100,200)"})
	require.NoError(t, err)

	for _, v := range []struct {
		at    time.Duration
		alpha uint8
		phase string
	}{
		{at: 0, alpha: 255, phase: "entry start: fully transparent"},
		{at: 50 * time.Millisecond, alpha: 128, phase: "entry middle: half transparent"},
		{at: 100 * time.Millisecond, alpha: 0, phase: "entry end: opaque"},
		{at: 500 * time.Millisecond, alpha: 0, phase: "hold: opaque"},
		{at: 800 * time.Millisecond, alpha: 0, phase: "exit start: opaque"},
		{at: 900 * time.Millisecond, alpha: 128, phase: "exit middle: half transparent"},
		{at: 1000 * time.Millisecond, alpha: 255, phase: "exit end: fully transparent"},
		{at: 1200 * time.Millisecond, alpha: 255, phase: "after the event: stays fully transparent"},
		{at: -100 * time.Millisecond, alpha: 255, phase: "before the event: fully transparent"},
	} {
		t.Run(v.phase, func(t *testing.T) {
			a, err := f.alphaAt(v.at, time.Second)
			require.NoError(t, err)
			assert.Equal(t, v.alpha, a)
		})
	}
}

// TestSSAFadeAlphaAtFADE uses the equivalent \fade sample and checks the same
// sample points plus the three-level-specific phases and endpoints
func TestSSAFadeAlphaAtFADE(t *testing.T) {
	f, err := parseSSAFadeTag(ssaOverrideTag{name: "fade", args: "(255,0,255,0,100,800,1000)"})
	require.NoError(t, err)

	for _, v := range []struct {
		at    time.Duration
		alpha uint8
	}{
		{at: 0, alpha: 255},
		{at: 50 * time.Millisecond, alpha: 128},
		{at: 100 * time.Millisecond, alpha: 0},
		{at: 500 * time.Millisecond, alpha: 0},
		{at: 800 * time.Millisecond, alpha: 0},
		{at: 900 * time.Millisecond, alpha: 128},
		{at: 1000 * time.Millisecond, alpha: 255},
		{at: 1200 * time.Millisecond, alpha: 255},
	} {
		a, err := f.alphaAt(v.at, time.Second)
		require.NoError(t, err)
		assert.Equal(t, v.alpha, a, "at %s", v.at)
	}

	// A non-trivial three-level fade, including its plateau endpoints
	custom, err := parseSSAFadeTag(ssaOverrideTag{name: "fade", args: "(128,16,200,100,200,700,900)"})
	require.NoError(t, err)
	for _, v := range []struct {
		at    time.Duration
		alpha uint8
	}{
		{at: 0, alpha: 128},                      // before t1: level 1
		{at: 100 * time.Millisecond, alpha: 128}, // t1 endpoint: still level 1
		{at: 150 * time.Millisecond, alpha: 72},  // mid transition, rounded
		{at: 200 * time.Millisecond, alpha: 16},  // t2: level 2
		{at: 500 * time.Millisecond, alpha: 16},  // plateau
		{at: 700 * time.Millisecond, alpha: 16},  // t3 endpoint: still level 2
		{at: 800 * time.Millisecond, alpha: 108}, // mid exit, rounded
		{at: 900 * time.Millisecond, alpha: 200}, // t4: level 3
		{at: 1000 * time.Millisecond, alpha: 200},
	} {
		a, err := custom.alphaAt(v.at, time.Second)
		require.NoError(t, err)
		assert.Equal(t, v.alpha, a, "at %s", v.at)
	}
}

func TestSSAInterpolateAlpha(t *testing.T) {
	// Endpoints
	assert.Equal(t, uint8(0), ssaInterpolateAlpha(0, 255, 0, 100, 0))
	assert.Equal(t, uint8(255), ssaInterpolateAlpha(0, 255, 0, 100, 100))
	// Rounding half away from zero
	assert.Equal(t, uint8(13), ssaInterpolateAlpha(0, 25, 0, 100, 50))
	assert.Equal(t, uint8(243), ssaInterpolateAlpha(255, 230, 0, 100, 50))
	// Zero-length transition keeps the starting alpha, never divides by zero
	assert.Equal(t, uint8(10), ssaInterpolateAlpha(10, 20, 100, 100, 100))
	assert.Equal(t, uint8(10), ssaInterpolateAlpha(10, 20, 100, 100, 150))
}

func TestItemSSAFade(t *testing.T) {
	itemWithEffect := func(effect string) *Item {
		return &Item{
			StartAt: time.Second,
			EndAt:   2 * time.Second,
			Lines: []Line{
				{Items: []LineItem{
					{InlineStyle: &StyleAttributes{SSAEffect: effect}, Text: "Hello"},
				}},
			},
		}
	}

	// No fade tag
	item := itemWithEffect(`{\i1}`)
	assert.Nil(t, item.SSAFade())
	assert.Empty(t, item.SSAFades())

	// One fad
	item = itemWithEffect(`{\fad(100,200)}Hello`)
	f := item.SSAFade()
	require.NotNil(t, f)
	assert.Equal(t, SSAFadeTypeFAD, f.Type)
	assert.Equal(t, 100, f.FadeIn)
	assert.Equal(t, 200, f.FadeOut)

	// Fade mixed with other override tags
	item = itemWithEffect(`{\an8\fad(100,200)\i1}`)
	f = item.SSAFade()
	require.NotNil(t, f)
	assert.Equal(t, SSAFadeTypeFAD, f.Type)

	// Conflicting tags: every valid tag is returned, the last one wins for SSAFade
	item = itemWithEffect(`{\fad(100,200)\fade(255,0,255,0,100,800,1000)}`)
	fades := item.SSAFades()
	require.Len(t, fades, 2)
	assert.Equal(t, SSAFadeTypeFAD, fades[0].Type)
	assert.Equal(t, SSAFadeTypeFADE, fades[1].Type)
	f = item.SSAFade()
	require.NotNil(t, f)
	assert.Equal(t, SSAFadeTypeFADE, f.Type)

	// A malformed tag is ignored structurally but remains in the text
	item = itemWithEffect(`{\fad(foo,200)}`)
	assert.Nil(t, item.SSAFade())
	assert.Equal(t, `{\fad(foo,200)}`, item.Lines[0].Items[0].InlineStyle.SSAEffect)

	// SSAAlphaAt resolves the relative fade times against the event boundaries
	item = itemWithEffect(`{\fad(100,200)}`)
	for _, v := range []struct {
		at    time.Duration
		alpha uint8
	}{
		{at: 1000 * time.Millisecond, alpha: 255},
		{at: 1100 * time.Millisecond, alpha: 0},
		{at: 1800 * time.Millisecond, alpha: 0},
		{at: 2000 * time.Millisecond, alpha: 255},
	} {
		a, err := item.SSAAlphaAt(v.at)
		require.NoError(t, err)
		assert.Equal(t, v.alpha, a, "at %s", v.at)
	}

	// No fade tag means fully opaque at every time
	item = itemWithEffect(`{\i1}`)
	a, err := item.SSAAlphaAt(1500 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint8(0), a)
}

func TestItemSetSSAFade(t *testing.T) {
	// Editing an existing tag keeps sibling tags, text and timing
	item := &Item{
		StartAt: time.Second,
		EndAt:   2 * time.Second,
		Lines: []Line{{Items: []LineItem{
			{InlineStyle: &StyleAttributes{SSAEffect: `{\an8\fad(100,200)\i1}`}, Text: "Hello"},
			{Text: " world"},
		}}},
	}
	require.NoError(t, item.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: 300, FadeOut: 400}))
	assert.Equal(t, `{\fad(300,400)\an8\i1}`, item.Lines[0].Items[0].InlineStyle.SSAEffect)
	assert.Equal(t, "Hello", item.Lines[0].Items[0].Text)
	assert.Equal(t, " world", item.Lines[0].Items[1].Text)
	assert.Equal(t, time.Second, item.StartAt)
	assert.Equal(t, 2*time.Second, item.EndAt)

	// Switching to a fade tag
	require.NoError(t, item.SetSSAFade(&SSAFade{
		Type:   SSAFadeTypeFADE,
		Alpha1: 255, Alpha2: 0, Alpha3: 255,
		Time1: 0, Time2: 100, Time3: 800, Time4: 1000,
	}))
	assert.Equal(t, `{\fade(255,0,255,0,100,800,1000)\an8\i1}`, item.Lines[0].Items[0].InlineStyle.SSAEffect)

	// Removing the fade keeps the other override tags
	require.NoError(t, item.SetSSAFade(nil))
	assert.Equal(t, `{\an8\i1}`, item.Lines[0].Items[0].InlineStyle.SSAEffect)

	// Removing the only tag empties the block
	onlyFade := &Item{Lines: []Line{{Items: []LineItem{
		{InlineStyle: &StyleAttributes{SSAEffect: `{\fad(100,200)}`}, Text: "Hello"},
	}}}}
	require.NoError(t, onlyFade.SetSSAFade(nil))
	assert.Equal(t, "", onlyFade.Lines[0].Items[0].InlineStyle.SSAEffect)

	// Adding a fade to an event without override blocks prepends one
	plain := &Item{Lines: []Line{{Items: []LineItem{{Text: "Hello"}}}}}
	require.NoError(t, plain.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: 100, FadeOut: 200}))
	assert.Equal(t, `{\fad(100,200)}`, plain.Lines[0].Items[0].InlineStyle.SSAEffect)
	assert.Equal(t, "Hello", plain.Lines[0].Items[0].Text)

	// Multiple fades collapse into the replacement, preserving the other tags
	multi := &Item{Lines: []Line{{Items: []LineItem{
		{InlineStyle: &StyleAttributes{SSAEffect: `{\fad(100,200)\i1}`}, Text: "Hello "},
		{InlineStyle: &StyleAttributes{SSAEffect: `{\fade(255,0,255,0,1,2,3,4)\b1}`}, Text: "world"},
	}}}}
	require.NoError(t, multi.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: 50, FadeOut: 50}))
	assert.Equal(t, `{\fad(50,50)\i1}`, multi.Lines[0].Items[0].InlineStyle.SSAEffect)
	assert.Equal(t, `{\b1}`, multi.Lines[0].Items[1].InlineStyle.SSAEffect)
	require.Len(t, multi.SSAFades(), 1)

	// Several override blocks in one effect: the replacement targets the block
	// holding the fade, the other block and the text between them survive
	separate := &Item{Lines: []Line{{Items: []LineItem{
		{InlineStyle: &StyleAttributes{SSAEffect: `{\i1}middle{\fad(100,200)}`}, Text: "Hello"},
	}}}}
	require.NoError(t, separate.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: 50, FadeOut: 50}))
	assert.Equal(t, `{\i1}middle{\fad(50,50)}`, separate.Lines[0].Items[0].InlineStyle.SSAEffect)
	require.NoError(t, separate.SetSSAFade(nil))
	assert.Equal(t, `{\i1}middle`, separate.Lines[0].Items[0].InlineStyle.SSAEffect)

	// Adding a fade to an effect whose blocks carry none prepends a new block
	noFade := &Item{Lines: []Line{{Items: []LineItem{
		{InlineStyle: &StyleAttributes{SSAEffect: `{\i1}middle`}, Text: "Hello"},
	}}}}
	require.NoError(t, noFade.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: 50, FadeOut: 50}))
	assert.Equal(t, `{\fad(50,50)}{\i1}middle`, noFade.Lines[0].Items[0].InlineStyle.SSAEffect)

	// Invalid fades are rejected without mutating the event
	invalid := &Item{Lines: []Line{{Items: []LineItem{
		{InlineStyle: &StyleAttributes{SSAEffect: `{\fad(100,200)}`}, Text: "Hello"},
	}}}}
	err := invalid.SetSSAFade(&SSAFade{Type: SSAFadeTypeFAD, FadeIn: -1})
	assert.Equal(t, ErrInvalidSSAFade, err)
	assert.Equal(t, `{\fad(100,200)}`, invalid.Lines[0].Items[0].InlineStyle.SSAEffect)
	err = invalid.SetSSAFade(&SSAFade{Type: SSAFadeTypeFADE, Time1: 10, Time2: 5})
	assert.Equal(t, ErrInvalidSSAFade, err)
}

func TestNewSSAFadeConstructors(t *testing.T) {
	fad, err := NewSSAFADFade(100*time.Millisecond, 200*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, &SSAFade{Type: SSAFadeTypeFAD, FadeIn: 100, FadeOut: 200}, fad)
	_, err = NewSSAFADFade(-time.Millisecond, 0)
	assert.Equal(t, ErrInvalidSSAFade, err)

	fade, err := NewSSAFADEFade(255, 0, 255, 0, 100*time.Millisecond, 800*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.Equal(t, &SSAFade{
		Type:   SSAFadeTypeFADE,
		Alpha1: 255, Alpha2: 0, Alpha3: 255,
		Time1: 0, Time2: 100, Time3: 800, Time4: 1000,
	}, fade)
	_, err = NewSSAFADEFade(0, 0, 0, 0, 200*time.Millisecond, 100*time.Millisecond, 0)
	assert.Equal(t, ErrInvalidSSAFade, err)
}
