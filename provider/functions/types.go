package functions

import "github.com/pulumi/pulumi-go-provider/infer"

// Attachment represents an email attachment for the sendEmail function.
type Attachment struct {
	Content     *string `pulumi:"content,optional" json:"content,omitempty"`
	Path        *string `pulumi:"path,optional" json:"path,omitempty"`
	Filename    string  `pulumi:"filename" json:"filename"`
	ContentType *string `pulumi:"contentType,optional" json:"content_type,omitempty"`
	ContentId   *string `pulumi:"contentId,optional" json:"content_id,omitempty"`
}

func (a *Attachment) Annotate(annotator infer.Annotator) {
	annotator.Describe(a, "An email attachment.")
	annotator.Describe(&a.Content, "Base64-encoded content of the attachment.")
	annotator.Describe(&a.Path, "URL path to fetch the attachment content from.")
	annotator.Describe(&a.Filename, "The filename of the attachment.")
	annotator.Describe(&a.ContentType, "The MIME type of the attachment.")
	annotator.Describe(&a.ContentId, "Content ID for inline attachments (e.g., for embedding images in HTML).")
}

// Tag represents an email tag for categorization and filtering.
type Tag struct {
	Name  string `pulumi:"name" json:"name"`
	Value string `pulumi:"value" json:"value"`
}

func (t *Tag) Annotate(annotator infer.Annotator) {
	annotator.Describe(t, "An email tag for categorization.")
	annotator.Describe(&t.Name, "The tag name.")
	annotator.Describe(&t.Value, "The tag value.")
}
