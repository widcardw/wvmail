package services

import (
	"bytes"
	"fmt"
	"mime"
	"net/mail"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Some providers (notably Tencent QQ/Exmail) omit Content-Disposition and the
// filename from BODYSTRUCTURE even though the actual MIME part headers contain
// them. A text attachment then looks exactly like a body part and disappears
// from the attachment list. Fetch BODY[part.MIME] for ambiguous leaf parts and
// merge the real headers back into the structure before classifying them.
func enrichBodyStructureMIMEHeaders(
	client *imapclient.Client,
	uidSet imap.UIDSet,
	structure imap.BodyStructure,
) error {
	var leaves []bodyStructureLeaf
	structure.Walk(func(path []int, part imap.BodyStructure) bool {
		single, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		if single.Disposition() == nil && single.Filename() == "" && strings.TrimSpace(single.ID) == "" {
			leaves = append(leaves, bodyStructureLeaf{path: append([]int(nil), path...), part: single})
		}
		return false
	})
	if len(leaves) == 0 {
		return nil
	}

	sections := make([]*imap.FetchItemBodySection, 0, len(leaves))
	leafByPath := make(map[string]*imap.BodyStructureSinglePart, len(leaves))
	for _, leaf := range leaves {
		section := &imap.FetchItemBodySection{
			Specifier: imap.PartSpecifierMIME,
			Part:      leaf.path,
			Peek:      true,
		}
		sections = append(sections, section)
		leafByPath[fmt.Sprint(leaf.path)] = leaf.part
	}

	fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{BodySection: sections})
	defer fetchCmd.Close()
	bufs, err := fetchCmd.Collect()
	if err != nil {
		return fmt.Errorf("fetch MIME headers: %w", err)
	}
	if len(bufs) == 0 {
		return nil
	}

	for _, section := range sections {
		raw := bufs[0].FindBodySection(section)
		if len(raw) == 0 {
			continue
		}
		part := leafByPath[fmt.Sprint(section.Part)]
		if part == nil {
			continue
		}
		header, err := parseMIMEPartHeader(raw)
		if err != nil {
			continue
		}
		mergeMIMEPartHeader(part, header)
	}
	return nil
}

type bodyStructureLeaf struct {
	path []int
	part *imap.BodyStructureSinglePart
}

type mimePartHeader struct {
	disposition *imap.BodyStructureDisposition
	filename    string
	name        string
}

func parseMIMEPartHeader(raw []byte) (mimePartHeader, error) {
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return mimePartHeader{}, err
	}
	var result mimePartHeader

	if value := message.Header.Get("Content-Type"); value != "" {
		_, params, err := mime.ParseMediaType(value)
		if err == nil {
			result.name = decodeMimeHeader(params["name"])
		}
	}

	if value := message.Header.Get("Content-Disposition"); value != "" {
		disposition, params, err := mime.ParseMediaType(value)
		if err == nil {
			decoded := make(map[string]string, len(params))
			for key, value := range params {
				decoded[key] = decodeMimeHeader(value)
			}
			result.disposition = &imap.BodyStructureDisposition{
				Value:  strings.ToLower(disposition),
				Params: decoded,
			}
			result.filename = decoded["filename"]
		}
	}
	if result.filename == "" {
		result.filename = result.name
	}
	return result, nil
}

func mergeMIMEPartHeader(part *imap.BodyStructureSinglePart, header mimePartHeader) {
	if part.Params == nil {
		part.Params = make(map[string]string)
	}
	if part.Params["name"] == "" && header.name != "" {
		part.Params["name"] = header.name
	}

	if part.Extended == nil {
		part.Extended = &imap.BodyStructureSinglePartExt{}
	}
	if part.Extended.Disposition == nil && header.disposition != nil {
		part.Extended.Disposition = header.disposition
	}
	if part.Extended.Disposition != nil && part.Extended.Disposition.Params == nil {
		part.Extended.Disposition.Params = make(map[string]string)
	}
	if part.Extended.Disposition != nil &&
		part.Extended.Disposition.Params["filename"] == "" &&
		header.filename != "" {
		part.Extended.Disposition.Params["filename"] = header.filename
	}
}
