package git

import (
	"fmt"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

type MimeType struct {
	ContentType string
}

func (mt MimeType) IsText() bool {
	return strings.Contains(mt.ContentType, "text/")
}

func (mt MimeType) IsCSV() bool {
	return strings.Contains(mt.ContentType, "text/csv")
}

func (mt MimeType) IsImage() bool {
	return strings.Contains(mt.ContentType, "image/")
}

func (mt MimeType) IsSVG() bool {
	return strings.Contains(mt.ContentType, "image/svg+xml")
}

func (mt MimeType) IsPDF() bool {
	return strings.Contains(mt.ContentType, "application/pdf")
}

func (mt MimeType) IsAudio() bool {
	return strings.Contains(mt.ContentType, "audio/")
}

func (mt MimeType) IsVideo() bool {
	return strings.Contains(mt.ContentType, "video/")
}

func (mt MimeType) CanBeHighlighted() bool {
	return mt.IsText() && !mt.IsCSV()
}

func (mt MimeType) CanBeEmbedded() bool {
	return mt.IsImage() || mt.IsPDF() || mt.IsAudio() || mt.IsVideo()
}

func (mt MimeType) CanBeRendered() bool {
	return mt.IsText() || mt.IsImage() || mt.IsSVG() || mt.IsPDF() || mt.IsAudio() || mt.IsVideo()
}

func (mt MimeType) RenderType() string {
	t := strings.Split(mt.ContentType, "/")
	str := ""
	if len(t) == 2 {
		str = fmt.Sprintf("(%s)", strings.ToUpper(t[1]))
	}

	// More user friendly description
	if mt.IsImage() || mt.IsSVG() {
		return fmt.Sprintf("Image %s", str)
	}
	if mt.IsAudio() {
		return fmt.Sprintf("Audio %s", str)
	}
	if mt.IsVideo() {
		return fmt.Sprintf("Video %s", str)
	}
	if mt.IsPDF() {
		return "PDF"
	}
	if mt.IsCSV() {
		return "CSV"
	}
	if mt.IsText() {
		return "Text"
	}
	return "Binary"
}

func DetectMimeType(data []byte) MimeType {
	return MimeType{mimetype.Detect(data).String()}
}
