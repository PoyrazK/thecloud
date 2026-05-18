package httphandlers

import (
	"strings"
	"testing"
)

func TestContentDispositionAttachment(t *testing.T) {
	cases := []struct {
		name             string
		key              string
		wantFilename     string
		wantFilenameStar string
	}{
		{
			name:             "simple ascii filename",
			key:              "report.pdf",
			wantFilename:     `filename="report.pdf"`,
			wantFilenameStar: `filename*=UTF-8''report.pdf`,
		},
		{
			name:             "nested key uses basename",
			key:              "exports/2026/q1/report.pdf",
			wantFilename:     `filename="report.pdf"`,
			wantFilenameStar: `filename*=UTF-8''report.pdf`,
		},
		{
			name:             "CRLF response splitting attempt is sanitized",
			key:              "evil\r\nSet-Cookie: pwned=1",
			wantFilename:     `filename="evil__Set-Cookie: pwned=1"`,
			wantFilenameStar: `filename*=UTF-8''evil%0D%0ASet-Cookie%3A%20pwned%3D1`,
		},
		{
			name:             "embedded quote and backslash are sanitized",
			key:              `bad"name\file.txt`,
			wantFilename:     `filename="bad_name_file.txt"`,
			wantFilenameStar: `filename*=UTF-8''bad%22name%5Cfile.txt`,
		},
		{
			name:             "non-ASCII falls back in legacy filename, preserved in filename*",
			key:              "résumé.pdf",
			wantFilename:     `filename="r__sum__.pdf"`, // 2 bytes per accented char both replaced
			wantFilenameStar: `filename*=UTF-8''r%C3%A9sum%C3%A9.pdf`,
		},
		{
			name:             "empty key falls back to download",
			key:              "",
			wantFilename:     `filename="download"`,
			wantFilenameStar: `filename*=UTF-8''download`,
		},
		{
			name:             "trailing slash falls back to download",
			key:              "folder/",
			wantFilename:     `filename="folder"`, // path.Base normalizes
			wantFilenameStar: `filename*=UTF-8''folder`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := contentDispositionAttachment(tc.key)
			if !strings.HasPrefix(got, "attachment; ") {
				t.Errorf("missing attachment prefix: %q", got)
			}
			if !strings.Contains(got, tc.wantFilename) {
				t.Errorf("missing legacy filename param\n got:  %q\n want: %q", got, tc.wantFilename)
			}
			if !strings.Contains(got, tc.wantFilenameStar) {
				t.Errorf("missing filename* param\n got:  %q\n want: %q", got, tc.wantFilenameStar)
			}
			if strings.ContainsAny(got, "\r\n") {
				t.Errorf("output must not contain CR/LF: %q", got)
			}
		})
	}
}

func TestContentDispositionAttachment_NoCRLFEverEscapes(t *testing.T) {
	for _, c := range []byte{'\r', '\n', 0, 0x1f, 0x7f} {
		key := "x" + string(c) + "y"
		got := contentDispositionAttachment(key)
		if strings.ContainsAny(got, "\r\n") {
			t.Fatalf("control byte 0x%02x leaked into header: %q", c, got)
		}
	}
}
