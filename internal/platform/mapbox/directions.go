package mapbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/yourusername/ghartak-backend/internal/platform/geo"
)

type directionsResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64         `json:"distance"`
		Duration float64         `json:"duration"`
		Geometry json.RawMessage `json:"geometry"`
	} `json:"routes"`
}

func (c *Client) Route(ctx context.Context, from, to Point) (Route, error) {
	if err := validPoints(from, to); err != nil {
		return Route{}, err
	}
	path := fmt.Sprintf("/directions/v5/mapbox/driving/%s;%s", encodeCoord(from), encodeCoord(to))
	query := url.Values{}
	query.Set("geometries", "geojson")
	query.Set("overview", "full")
	query.Set("alternatives", "false")
	query.Set("steps", "false")
	body, err := c.get(ctx, path, query)
	if err != nil {
		return Route{}, err
	}
	return parseDirections(body)
}

func parseDirections(body []byte) (Route, error) {
	var resp directionsResponse
	if err := decodeJSON(body, &resp); err != nil {
		return Route{}, err
	}
	if resp.Code != "Ok" || len(resp.Routes) == 0 {
		return Route{}, ErrNoRoute
	}
	route := resp.Routes[0]
	return Route{
		DistanceM: int(route.Distance + 0.5),
		DurationS: int(route.Duration + 0.5),
		Geometry:  route.Geometry,
	}, nil
}

func validPoints(from, to Point) error {
	if !geo.ValidCoord(from.Lat, from.Lng) || !geo.ValidCoord(to.Lat, to.Lng) {
		return ErrBadInput
	}
	return nil
}
