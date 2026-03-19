package api
import "embed"

//go:embed pkg/api/index.html
//go:embed openapi.yaml
var swaggerUI embed.FS