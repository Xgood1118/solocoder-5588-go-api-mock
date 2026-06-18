package models

import "time"

type HAR struct {
	Log HARLog `json:"log"`
}

type HARLog struct {
	Version string         `json:"version"`
	Creator HARCreator     `json:"creator"`
	Entries []HAREntry     `json:"entries"`
}

type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HAREntry struct {
	StartedDateTime time.Time   `json:"startedDateTime"`
	Time            int         `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Timings         HARTimings  `json:"timings"`
	ServerIPAddress string      `json:"serverIPAddress,omitempty"`
	Connection      string      `json:"connection,omitempty"`
}

type HARRequest struct {
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	HTTPVersion string              `json:"httpVersion"`
	Headers     []HARNameValue      `json:"headers"`
	QueryString []HARNameValue      `json:"queryString"`
	PostData    *HARPostData        `json:"postData,omitempty"`
	HeadersSize int                 `json:"headersSize"`
	BodySize    int                 `json:"bodySize"`
}

type HARResponse struct {
	Status      int                 `json:"status"`
	StatusText  string              `json:"statusText"`
	HTTPVersion string              `json:"httpVersion"`
	Headers     []HARNameValue      `json:"headers"`
	Content     HARContent          `json:"content"`
	HeadersSize int                 `json:"headersSize"`
	BodySize    int                 `json:"bodySize"`
}

type HARNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HARPostData struct {
	MimeType string          `json:"mimeType"`
	Text     string          `json:"text"`
	Params   []HARPostParam  `json:"params,omitempty"`
}

type HARPostParam struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type HARContent struct {
	Size        int    `json:"size"`
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`
	Compression int    `json:"compression,omitempty"`
}

type HARTimings struct {
	Blocked int `json:"blocked"`
	DNS     int `json:"dns"`
	Connect int `json:"connect"`
	Send    int `json:"send"`
	Wait    int `json:"wait"`
	Receive int `json:"receive"`
	SSL     int `json:"ssl,omitempty"`
}

type RequestSummary struct {
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	HeaderKeys    []string          `json:"header_keys"`
	BodyPreview   string            `json:"body_preview"`
	BodySHA256    string            `json:"body_sha256"`
	BodySize      int64             `json:"body_size"`
	Timestamp     time.Time         `json:"timestamp"`
	FullBodySaved bool              `json:"full_body_saved"`
}
