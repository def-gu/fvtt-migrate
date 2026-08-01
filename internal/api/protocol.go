package api

import "github.com/def-gu/fvtt-migrate/internal/content"

const (
	PathHello   = "/v1/hello"
	PathMissing = "/v1/blobs/missing"
	PathBlob    = "/v1/blobs/"
	PathPlace   = "/v1/tree/place"
	PathCommit  = "/v1/commit"
)

var Capabilities = []string{"target/1", "digest/blake3-256", "transfer/whole-file"}

type Hello struct {
	Agent        string   `json:"agent"`
	Capabilities []string `json:"capabilities"`
	Root         string   `json:"root"`
}

type MissingRequest struct {
	Digests []content.Digest `json:"digests"`
}

type MissingResponse struct {
	Missing []content.Digest `json:"missing"`
}

type PlaceRequest struct {
	Digest content.Digest `json:"digest"`
	Path   string         `json:"path"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
