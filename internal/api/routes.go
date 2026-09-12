package api

// CalledRoutes is every route the CLI is capable of reaching, as a method and
// the spec's own path template.
//
// Maintained by hand rather than derived, because deriving it would mean
// parsing the CLI's own source, and a list you have to remember to update is
// exactly what the accompanying test enforces. Adding a command without adding
// its route here fails the build.
var CalledRoutes = []struct{ Method, Path string }{
	{"GET", "/v1/me"},
	{"GET", "/v1/me/usage"},
	{"GET", "/v1/links"},
	{"POST", "/v1/links"},
	{"GET", "/v1/links/{id}"},
	{"DELETE", "/v1/links/{id}"},
	{"GET", "/v1/links/{id}/content"},
	{"PUT", "/v1/links/{id}/tags"},
	{"GET", "/v1/collections"},
	{"POST", "/v1/collections"},
	{"DELETE", "/v1/collections/{id}"},
	{"GET", "/v1/tags"},
	{"POST", "/v1/tags"},
	{"DELETE", "/v1/tags/{id}"},
	{"GET", "/v1/highlights"},
	{"POST", "/v1/highlights"},
	{"GET", "/v1/search"},
	{"GET", "/v1/search/highlights"},
	{"GET", "/v1/discover"},
	{"POST", "/v1/discover/{id}/save"},
	{"GET", "/v1/feeds"},
	{"POST", "/v1/feeds"},
	{"DELETE", "/v1/feeds/{id}"},
	{"GET", "/v1/feeds/export"},
}
