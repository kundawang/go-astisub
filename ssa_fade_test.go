package astisub_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ssaFadeEventFormat = "Format: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"

func readSSAFadeEvent(t *testing.T, text string) *astisub.Subtitles {
	t.Helper()
	s, err := astisub.ReadFromSSA(strings.NewReader("[Events]\n" + ssaFadeEventFormat + "Dialogue: Marked=0,0:00:01.00,0:00:02.00,,Cher,0,0,0,," + text + "\n"))
	require.NoError(t, err)
	require.Len(t, s.Items, 1)
	return s
}

// TestSSAFadeReadWriteRoundTrip checks that \fad and \fade tags are read into
// the structured representation, edited and written back as legal ASS while
// the sibling override tags, the text and the line timing stay untouched
func TestSSAFadeReadWriteRoundTrip(t *testing.T) {
	for _, text := range []string{
		`{\an8\fad(100,200)\i1}Hello world`,
		`{\fade(255,0,255,0,100,800,1000)}Hello world`,
	} {
		s := readSSAFadeEvent(t, text)
		item := s.Items[0]

		// Line timing is preserved
		assert.Equal(t, time.Second, item.StartAt)
		assert.Equal(t, 2*time.Second, item.EndAt)
		assert.Equal(t, "Hello world", item.String())

		// Structured read
		f := item.SSAFade()
		require.NotNil(t, f)
		assert.Len(t, item.SSAFades(), 1)

		// Write back untouched: the event text round-trips verbatim
		w := &bytes.Buffer{}
		require.NoError(t, s.WriteToSSA(w))
		assert.Contains(t, w.String(), text)
	}

	// Edit the fade in place, then write back: sibling tags and text survive
	s := readSSAFadeEvent(t, `{\an8\fad(100,200)\i1}Hello world`)
	require.NoError(t, s.Items[0].SetSSAFade(&astisub.SSAFade{
		Type:   astisub.SSAFadeTypeFADE,
		Alpha1: 255, Alpha2: 0, Alpha3: 255,
		Time1: 0, Time2: 100, Time3: 800, Time4: 1000,
	}))
	w := &bytes.Buffer{}
	require.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `{\fade(255,0,255,0,100,800,1000)\an8\i1}Hello world`)

	// The rewritten document parses again and exposes the edited fade
	reread, err := astisub.ReadFromSSA(bytes.NewReader(w.Bytes()))
	require.NoError(t, err)
	f := reread.Items[0].SSAFade()
	require.NotNil(t, f)
	assert.Equal(t, astisub.SSAFadeTypeFADE, f.Type)
	assert.Equal(t, uint8(255), f.Alpha1)
	assert.Equal(t, 1000, f.Time4)

	// Removing the fade keeps the rest of the block and the text
	require.NoError(t, reread.Items[0].SetSSAFade(nil))
	w = &bytes.Buffer{}
	require.NoError(t, reread.WriteToSSA(w))
	assert.Contains(t, w.String(), `{\an8\i1}Hello world`)
}

// TestSSAFadeAlphaSample checks the requirement's exact one-second sample with
// both the \fad form and its equivalent \fade form
func TestSSAFadeAlphaSample(t *testing.T) {
	events := map[string]*astisub.Subtitles{
		"fad":  readSSAFadeEvent(t, `{\fad(100,200)}Hello`),
		"fade": readSSAFadeEvent(t, `{\fade(255,0,255,0,100,800,1000)}Hello`),
	}
	for name, s := range events {
		item := s.Items[0]
		for _, v := range []struct {
			at    time.Duration
			alpha uint8
		}{
			{at: 1000 * time.Millisecond, alpha: 255}, // 0ms in: fully transparent
			{at: 1100 * time.Millisecond, alpha: 0},   // 100ms in: opaque
			{at: 1800 * time.Millisecond, alpha: 0},   // 800ms in: opaque
			{at: 2000 * time.Millisecond, alpha: 255}, // 1000ms in: fully transparent
		} {
			a, err := item.SSAAlphaAt(v.at)
			require.NoError(t, err)
			assert.Equal(t, v.alpha, a, "%s at %s", name, v.at)
		}
	}
}

// TestSSAFadeConflictsAndInvalid checks the deterministic behavior with
// conflicting tags and illegal parameters
func TestSSAFadeConflictsAndInvalid(t *testing.T) {
	// Two conflicting fades: both are exposed, the last one drives the alpha,
	// and both round-trip untouched when nothing is edited
	s := readSSAFadeEvent(t, `{\fad(100,200)\fade(128,16,200,100,200,700,900)}Hello`)
	item := s.Items[0]
	require.Len(t, item.SSAFades(), 2)
	assert.Equal(t, astisub.SSAFadeTypeFADE, item.SSAFade().Type)
	a, err := item.SSAAlphaAt(1000 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint8(128), a) // first level of the winning \fade tag

	w := &bytes.Buffer{}
	require.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `{\fad(100,200)\fade(128,16,200,100,200,700,900)}Hello`)

	// An illegal tag is ignored structurally but preserved verbatim in the text
	s = readSSAFadeEvent(t, `{\fad(foo,200)}Hello`)
	assert.Nil(t, s.Items[0].SSAFade())
	a, err = s.Items[0].SSAAlphaAt(1500 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint8(0), a) // no valid fade: fully opaque
	w = &bytes.Buffer{}
	require.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `{\fad(foo,200)}Hello`)

	// An event without any effect stays fully opaque and writes no block
	s = readSSAFadeEvent(t, `Hello world`)
	assert.Nil(t, s.Items[0].SSAFade())
	a, err = s.Items[0].SSAAlphaAt(1500 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint8(0), a)
}
