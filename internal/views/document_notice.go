package views

import "strings"

// Keep presentation metadata in its source language. A document can outlive a
// language change without recreating its editor or changing user-authored text.
type documentNoticeState struct {
	noticeParts    []documentNoticeMessage
	readOnlyReason string
}

type documentNoticeMessage struct {
	source string
	args   []any
}

// Only explicitly marked product words (for example the old/new diff side) are
// translated inside arguments. Paths, fingerprints and errors stay verbatim.
type noticeWord string

func noticeMessage(source string, args ...any) documentNoticeMessage {
	return documentNoticeMessage{source: source, args: args}
}

func (m documentNoticeMessage) text(locale string) string {
	args := append([]any(nil), m.args...)
	for i, arg := range args {
		if word, ok := arg.(noticeWord); ok {
			args[i] = translate(locale, string(word))
		}
	}
	return translate(locale, m.source, args...)
}

func (d *document) setNotice(parts ...documentNoticeMessage) {
	d.noticeParts = append([]documentNoticeMessage(nil), parts...)
	// Preserve an English representation for existing document metadata users.
	d.Notice = d.displayNotice("en")
}

func (d *document) noticeMessages() []documentNoticeMessage {
	if len(d.noticeParts) != 0 {
		return append([]documentNoticeMessage(nil), d.noticeParts...)
	}
	return []documentNoticeMessage{noticeMessage(d.Notice)}
}

func (d *document) displayNotice(locale string) string {
	parts := d.noticeMessages()
	labels := make([]string, len(parts))
	for i, part := range parts {
		labels[i] = part.text(locale)
	}
	return strings.Join(labels, " · ")
}

func (d *document) displayVersionNotice(locale string) string {
	if d.VersionNotice == "Read-only" && d.readOnlyReason != "" {
		return translate(locale, "Read-only · %s", d.readOnlyReason)
	}
	return translate(locale, d.VersionNotice)
}
