package api

import "github.com/def-gu/fvtt-migrate/internal/content"

const (
	PathHello     = "/v1/hello"
	PathMissing   = "/v1/blobs/missing"
	PathBlob      = "/v1/blobs/"
	PathPlace     = "/v1/tree/place"
	PathPlaceMany = "/v1/tree/place-many"
	PathCommit    = "/v1/commit"
	PathWorlds    = "/v1/worlds"
)

const CapPlaceMany = "transfer/place-many"

var Capabilities = []string{"target/1", "digest/blake3-256", "transfer/whole-file", CapPlaceMany}

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

// One request per file cost a round trip per file, which over a home connection
// turned laying out ninety thousand files into hours of latency.
type PlaceManyRequest struct {
	Entries []PlaceRequest `json:"entries"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorldCount struct {
	ID        string `json:"id"`
	Documents int    `json:"documents"`
	Failure   string `json:"failure,omitempty"`
}

type WorldsResponse struct {
	Worlds []WorldCount `json:"worlds"`
}
