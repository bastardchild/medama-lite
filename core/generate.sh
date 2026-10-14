#!/bin/bash

# Generate handlers from the OpenAPI specification using ogen-go.
go tool github.com/ogen-go/ogen/cmd/ogen --target api --clean openapi.yaml

# The browser Beacon API sends all content in text/plain even if the content is in JSON.
# As ogen-go validates the Content-Type header from our OpenAPI specification, this is a
# workaround to patch api/oas_request_decoders_gen.go to add text/plain to the list of
# accepted content types for the /event/hit endpoint and still use the same JSON decoder.

# This runs perl to match decodePostEventHitRequest specifically and patch accepted content types.
perl -0777 -i -pe 's/(func \(s \*Server\) decodePostEventHitRequest[\s\S]*?switch \{\n\s*case ct == "application\/json"):\n/\1, ct == "text\/plain":\n/g' ./api/oas_request_decoders_gen.go
