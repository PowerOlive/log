package log

import (
	"fmt"
	"os"
	"testing"
)

func TestConsoleWriter(t *testing.T) {
	w := &ConsoleWriter{}

	for _, level := range []string{"debug", "info", "warning", "error", "fatal", "panic", "hahaha"} {
		_, err := fmt.Fprintf(w, `{"time":"2019-07-10T05:35:54.277Z","level":"%s","caller":"test.go:42","error":"i am test error","foo":"bar","n":42,"message":"hello json console writer"}`+"\n", level)
		if err != nil {
			t.Errorf("test json console writer error: %+v", err)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	file, _ := os.Open(os.DevNull)
	if IsTerminal(file.Fd()) {
		t.Errorf("test is terminal mode for %s failed", os.DevNull)
	}
}

func TestConsoleWriterColor(t *testing.T) {
	w := &ConsoleWriter{
		ANSIColor: true,
	}

	for _, level := range []string{"debug", "info", "warning", "error", "fatal", "panic", "hahaha"} {
		_, err := fmt.Fprintf(w, `{"time":"2019-07-10T05:35:54.277Z","level":"%s","caller":"pretty.go:42","error":"i am test error","foo":"bar","n":42,"message":"hello json console color writer"}`+"\n", level)
		if err != nil {
			t.Errorf("test json color console writer error: %+v", err)
		}
	}
}

func TestConsoleWriterNewline(t *testing.T) {
	w := &ConsoleWriter{
		ANSIColor: true,
	}

	_, err := fmt.Fprintf(w, `{"time":"2019-07-10T05:35:54.277Z","level":"info","caller":"pretty.go:42","error":"i am test error","foo":"bar","n":42,"message":"hello json console color writer\n"}`)
	if err != nil {
		t.Errorf("test plain text console writer error: %+v", err)
	}
}

func TestConsoleWriterInvaild(t *testing.T) {
	w := &ConsoleWriter{
		ANSIColor: true,
	}

	_, err := fmt.Fprintf(w, "a long long long long plain text")
	if err != nil {
		t.Errorf("test plain text console writer error: %+v", err)
	}
}
