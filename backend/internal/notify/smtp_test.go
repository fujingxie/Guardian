package notify

import (
	"strings"
	"testing"
)

func TestEmailHeaderInjectionFilter(t *testing.T) {
	replacer := strings.NewReplacer("\r", "", "\n", "")

	badTo := "target@example.com\r\nBcc: victim@example.com"
	badSubject := "Warning Alert\r\nContent-Transfer-Encoding: base64"
	badFrom := "Admin\r\nReply-To: attacker@example.com"

	cleanTo := replacer.Replace(badTo)
	cleanSubject := replacer.Replace(badSubject)
	cleanFrom := replacer.Replace(badFrom)

	if strings.Contains(cleanTo, "\r") || strings.Contains(cleanTo, "\n") {
		t.Errorf("expected cleanTo to contain no CRLF characters")
	}
	if strings.Contains(cleanSubject, "\r") || strings.Contains(cleanSubject, "\n") {
		t.Errorf("expected cleanSubject to contain no CRLF characters")
	}
	if strings.Contains(cleanFrom, "\r") || strings.Contains(cleanFrom, "\n") {
		t.Errorf("expected cleanFrom to contain no CRLF characters")
	}

	expectedTo := "target@example.comBcc: victim@example.com"
	if cleanTo != expectedTo {
		t.Errorf("expected %q, got %q", expectedTo, cleanTo)
	}
}
