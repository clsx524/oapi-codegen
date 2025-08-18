package server

import (
	"net/http"
	"sort"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/oapi-codegen/oapi-codegen/v2/examples/authenticated-api/echo/api"
)

type server struct {
	sync.RWMutex
	lastID int64
	things map[int64]api.Thing
}

func NewServer() *server {
	return &server{
		lastID: 0,
		things: make(map[int64]api.Thing),
	}
}

func CreateMiddleware(v JWSValidator) ([]echo.MiddlewareFunc, error) {
	// Create authentication middleware for libopenapi
	authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Determine required scopes based on path and method
			requiredScopes := getRequiredScopes(c.Request().Method, c.Request().URL.Path)
			
			// If no authentication is required for this endpoint, continue
			if requiredScopes == nil {
				return next(c)
			}

			// Extract JWT from Authorization header
			jws, err := GetJWSFromRequest(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "Missing or invalid authorization header")
			}

			// Validate the JWT
			token, err := v.ValidateJWS(jws)
			if err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "Invalid JWT token")
			}

			// Check that token claims match required scopes
			err = CheckTokenClaims(requiredScopes, token)
			if err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
			}

			// Set the JWT claims in the context for handlers to access
			c.Set(JWTClaimsContextKey, token)

			return next(c)
		}
	}

	return []echo.MiddlewareFunc{authMiddleware}, nil
}

// getRequiredScopes determines what scopes are required for a given path and method
func getRequiredScopes(method, path string) []string {
	// Based on the API spec:
	// - GET /things requires authentication but no specific scopes (global security)
	// - POST /things requires authentication with "things:w" scope
	
	if path == "/things" {
		switch method {
		case "GET":
			return []string{} // Requires auth but no specific scopes
		case "POST":
			return []string{"things:w"} // Requires "things:w" scope
		}
	}
	
	// No authentication required for unknown paths
	return nil
}

// Ensure that we implement the server interface
var _ api.ServerInterface = (*server)(nil)

func (s *server) ListThings(ctx echo.Context) error {
	// This handler will only be called when a valid JWT is presented for
	// access.
	s.RLock()

	thingKeys := make([]int64, 0, len(s.things))
	for key := range s.things {
		thingKeys = append(thingKeys, key)
	}
	sort.Sort(int64s(thingKeys))

	things := make([]api.ThingWithID, 0, len(s.things))

	for _, key := range thingKeys {
		thing := s.things[key]
		things = append(things, api.ThingWithID{
			ID:   key,
			Name: thing.Name,
		})
	}

	s.RUnlock()

	return ctx.JSON(http.StatusOK, things)
}

type int64s []int64

func (in int64s) Len() int {
	return len(in)
}

func (in int64s) Less(i, j int) bool {
	return in[i] < in[j]
}

func (in int64s) Swap(i, j int) {
	in[i], in[j] = in[j], in[i]
}

var _ sort.Interface = (int64s)(nil)

func (s *server) AddThing(ctx echo.Context) error {
	// This handler will only be called when the JWT is valid and the JWT contains
	// the scopes required.
	var thing api.Thing
	err := ctx.Bind(&thing)
	if err != nil {
		return returnError(ctx, http.StatusBadRequest, "could not bind request body")
	}

	s.Lock()
	defer s.Unlock()

	s.things[s.lastID] = thing
	thingWithId := api.ThingWithID{
		Name: thing.Name,
		ID:   s.lastID,
	}
	s.lastID++

	return ctx.JSON(http.StatusCreated, thingWithId)
}

func returnError(ctx echo.Context, code int, message string) error {
	errResponse := api.Error{
		Code:    int32(code),
		Message: message,
	}
	return ctx.JSON(code, errResponse)
}
