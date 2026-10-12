// Package geocode 代理地名服务，可插拔。
//
// 为什么经后端转发：浏览器直连这些服务无法声明合规的 User-Agent，也会遇到跨域问题。
// 后端只做转发，**不落盘缓存**——缓存责任交给浏览器（响应带 Cache-Control），
// 这样服务端保持无状态。
//
// 实测中国大陆网络下 Nominatim 不可达而 Photon 可达且原生返回中文地名，故默认 Photon。
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Carey996/travel_map/api/internal/config"
)

// Result 是反向地理编码的归一化结果。
type Result struct {
	Lat         float64        `json:"lat"`
	Lng         float64        `json:"lng"`
	Name        string         `json:"name"`
	ShortName   string         `json:"short_name"`
	DisplayName string         `json:"display_name"`
	Kind        string         `json:"kind"`
	Country     string         `json:"country"`
	Address     map[string]any `json:"address"`
}

// SearchItem 是地名搜索的单条结果。
type SearchItem struct {
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Kind        string  `json:"kind"`
	Category    string  `json:"category"`
	Type        string  `json:"type"`
}

// Geocoder 依据配置选择 Photon 或 Nominatim。
type Geocoder struct {
	provider    string
	baseURL     string
	client      *http.Client
	minInterval time.Duration

	// 轻量节流器，纯粹为遵守对方的使用条款，不属于业务状态
	mu       sync.Mutex
	lastCall time.Time
}

// New 构造地名服务客户端。
func New(cfg *config.Config) *Geocoder {
	interval := 150 * time.Millisecond // Photon
	if cfg.GeocodeProvider == "nominatim" {
		interval = 1050 * time.Millisecond // Nominatim 条款要求每秒最多 1 次
	}
	return &Geocoder{
		provider:    cfg.GeocodeProvider,
		baseURL:     strings.TrimRight(cfg.GeoBaseURL(), "/"),
		client:      &http.Client{Timeout: cfg.GeocodeTimeout},
		minInterval: interval,
	}
}

func (g *Geocoder) throttle(ctx context.Context) {
	g.mu.Lock()
	wait := g.minInterval - time.Since(g.lastCall)
	g.mu.Unlock()
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
	g.mu.Lock()
	g.lastCall = time.Now()
	g.mu.Unlock()
}

func (g *Geocoder) do(ctx context.Context, path string, params url.Values) ([]byte, error) {
	g.throttle(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("构造地名请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if g.provider == "nominatim" {
		req.Header.Set("User-Agent", "travel-blog/0.2 (personal project)")
		// 注意：Photon 一旦收到 Accept-Language 就会切换到翻译后的英文名，
		// 而我们要的正是本地语言地名（中文/日文），所以只给 Nominatim 发这个头
		req.Header.Set("Accept-Language", "zh-CN,zh,en")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求地名服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("地名服务返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取地名响应失败: %w", err)
	}
	return body, nil
}

// Reverse 由经纬度反查地名。
func (g *Geocoder) Reverse(ctx context.Context, lat, lng float64) (*Result, error) {
	if g.provider == "nominatim" {
		params := url.Values{}
		params.Set("format", "jsonv2")
		params.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
		params.Set("lon", strconv.FormatFloat(lng, 'f', 6, 64))
		params.Set("zoom", "14")
		params.Set("addressdetails", "1")
		body, err := g.do(ctx, "/reverse", params)
		if err != nil {
			return nil, err
		}
		return normalizeNominatimReverse(body, lat, lng)
	}

	params := url.Values{}
	params.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	params.Set("lon", strconv.FormatFloat(lng, 'f', 6, 64))
	body, err := g.do(ctx, "/reverse", params)
	if err != nil {
		return nil, err
	}
	return normalizePhotonReverse(body, lat, lng)
}

// Search 按关键字搜索地名。
func (g *Geocoder) Search(ctx context.Context, query string, limit int) ([]SearchItem, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	if g.provider == "nominatim" {
		params.Set("format", "jsonv2")
		params.Set("q", query)
		params.Set("addressdetails", "1")
		body, err := g.do(ctx, "/search", params)
		if err != nil {
			return nil, err
		}
		return normalizeNominatimSearch(body)
	}
	params.Set("q", query)
	body, err := g.do(ctx, "/api", params)
	if err != nil {
		return nil, err
	}
	return normalizePhotonSearch(body)
}

// ---------------------------------------------------------------------------
// Photon
// ---------------------------------------------------------------------------

type photonResponse struct {
	Features []struct {
		Properties map[string]any `json:"properties"`
		Geometry   struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func normalizePhotonReverse(body []byte, lat, lng float64) (*Result, error) {
	var parsed photonResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析地名响应失败: %w", err)
	}
	if len(parsed.Features) == 0 {
		return coordinateFallback(lat, lng), nil
	}
	f := parsed.Features[0]
	if len(f.Geometry.Coordinates) >= 2 {
		lng, lat = f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
	}
	return buildPhotonResult(f.Properties, lat, lng), nil
}

func normalizePhotonSearch(body []byte) ([]SearchItem, error) {
	var parsed photonResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析地名响应失败: %w", err)
	}
	out := make([]SearchItem, 0, len(parsed.Features))
	for _, f := range parsed.Features {
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}
		lng, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		res := buildPhotonResult(f.Properties, lat, lng)
		out = append(out, SearchItem{
			Lat: res.Lat, Lng: res.Lng, Name: res.Name,
			DisplayName: res.DisplayName, Kind: res.Kind,
			Category: str(props(f.Properties, "osm_key")),
			Type:     firstNonEmpty(str(props(f.Properties, "osm_value")), str(props(f.Properties, "type"))),
		})
	}
	return out, nil
}

func buildPhotonResult(p map[string]any, lat, lng float64) *Result {
	name := str(props(p, "name"))
	city := str(props(p, "city"))
	district := str(props(p, "district"))
	locality := str(props(p, "locality"))
	state := str(props(p, "state"))
	country := str(props(p, "country"))

	short := firstNonEmpty(name, city, district, locality, state, country)

	// 荒野等无人区可能没有任何名称，退化为坐标，保证前端总有东西可显示
	if short == "" {
		short = fmt.Sprintf("%.4f, %.4f", lat, lng)
	}

	parts := make([]string, 0, 6)
	for _, v := range []string{name, locality, district, city, state, country} {
		if v != "" && !contains(parts, v) {
			parts = append(parts, v)
		}
	}

	display := strings.Join(parts, ", ")
	if display == "" {
		display = short
	}

	full := short
	if country != "" && !strings.Contains(short, country) {
		full = short + " · " + country
	}

	return &Result{
		Lat: lat, Lng: lng,
		Name: full, ShortName: short, DisplayName: display,
		Kind:    classify(str(props(p, "osm_key")), str(props(p, "osm_value"))),
		Country: country,
		Address: map[string]any{
			"name": name, "locality": locality, "district": district, "city": city,
			"state": state, "country": country,
			"postcode": str(props(p, "postcode")), "street": str(props(p, "street")),
			"osm_type": str(props(p, "osm_type")), "osm_value": str(props(p, "osm_value")),
		},
	}
}

// ---------------------------------------------------------------------------
// Nominatim
// ---------------------------------------------------------------------------

func normalizeNominatimReverse(body []byte, lat, lng float64) (*Result, error) {
	var parsed struct {
		DisplayName string         `json:"display_name"`
		Address     map[string]any `json:"address"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析地名响应失败: %w", err)
	}
	if parsed.Address == nil {
		return coordinateFallback(lat, lng), nil
	}

	short := pickShortName(parsed.Address)
	country := str(props(parsed.Address, "country"))
	full := short
	if country != "" && !strings.Contains(short, country) {
		full = strings.TrimSpace(short + " · " + country)
	}
	if full == "" {
		full = fmt.Sprintf("%.4f, %.4f", lat, lng)
	}
	return &Result{
		Lat: lat, Lng: lng,
		Name: full, ShortName: short, DisplayName: parsed.DisplayName,
		Kind: classifyAddress(parsed.Address), Country: country, Address: parsed.Address,
	}, nil
}

func normalizeNominatimSearch(body []byte) ([]SearchItem, error) {
	var parsed []struct {
		Lat         string         `json:"lat"`
		Lon         string         `json:"lon"`
		Name        string         `json:"name"`
		DisplayName string         `json:"display_name"`
		Category    string         `json:"category"`
		Type        string         `json:"type"`
		Address     map[string]any `json:"address"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析地名响应失败: %w", err)
	}
	out := make([]SearchItem, 0, len(parsed))
	for _, item := range parsed {
		lat, _ := strconv.ParseFloat(item.Lat, 64)
		lng, _ := strconv.ParseFloat(item.Lon, 64)
		name := firstNonEmpty(item.Name, pickShortName(item.Address), item.DisplayName)
		out = append(out, SearchItem{
			Lat: lat, Lng: lng, Name: name, DisplayName: item.DisplayName,
			Kind: classifyAddress(item.Address), Category: item.Category, Type: item.Type,
		})
	}
	return out, nil
}

// namePriority 决定取简称时的优先级：越靠前越具体且越像「地名」。
var namePriority = []string{
	"tourism", "attraction", "historic", "leisure", "natural",
	"city", "town", "village", "municipality", "hamlet",
	"suburb", "neighbourhood", "quarter", "city_district",
	"county", "state_district", "state", "province", "country",
}

var cityKeys = []string{"city", "town", "village", "municipality", "hamlet"}
var spotKeys = []string{"tourism", "attraction", "historic", "leisure", "natural", "amenity", "building"}

func pickShortName(address map[string]any) string {
	for _, k := range namePriority {
		if v := str(props(address, k)); v != "" {
			return v
		}
	}
	return ""
}

func classify(osmKey, osmValue string) string {
	switch osmKey {
	case "tourism", "historic", "leisure", "natural", "amenity", "shop", "office", "highway", "railway", "waterway":
		return "spot"
	case "place":
		if contains(cityKeys, osmValue) {
			return "city"
		}
	case "boundary":
		if osmValue == "administrative" {
			return "region"
		}
	}
	return "other"
}

func classifyAddress(address map[string]any) string {
	for _, k := range spotKeys {
		if str(props(address, k)) != "" {
			return "spot"
		}
	}
	for _, k := range cityKeys {
		if str(props(address, k)) != "" {
			return "city"
		}
	}
	if str(props(address, "state")) != "" || str(props(address, "country")) != "" {
		return "region"
	}
	return "other"
}

func coordinateFallback(lat, lng float64) *Result {
	name := fmt.Sprintf("%.4f, %.4f", lat, lng)
	return &Result{
		Lat: lat, Lng: lng, Name: name, ShortName: name, DisplayName: name,
		Kind: "other", Address: map[string]any{},
	}
}

func props(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func str(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
