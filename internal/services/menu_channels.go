package services

import "fmt"

// DefaultMenuAvailableChannels is applied when channels are omitted on create
// or when existing rows have null/empty available_channels.
var DefaultMenuAvailableChannels = []string{
	"dine_in",
	"counter_eat_here",
	"counter_takeaway",
	"swiggy",
	"zomato",
}

var allowedMenuChannels = map[string]struct{}{
	"dine_in":          {},
	"counter_eat_here": {},
	"counter_takeaway": {},
	"swiggy":           {},
	"zomato":           {},
}

// NormalizeMenuAvailableChannels validates and de-duplicates channel ids.
// Empty input returns the default all-channels list when useDefaultOnEmpty is true;
// otherwise returns an empty slice.
func NormalizeMenuAvailableChannels(channels []string, useDefaultOnEmpty bool) ([]string, error) {
	if len(channels) == 0 {
		if useDefaultOnEmpty {
			out := make([]string, len(DefaultMenuAvailableChannels))
			copy(out, DefaultMenuAvailableChannels)
			return out, nil
		}
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(channels))
	out := make([]string, 0, len(channels))
	for _, ch := range channels {
		if _, ok := allowedMenuChannels[ch]; !ok {
			return nil, fmt.Errorf("invalid available_channels value: %s", ch)
		}
		if _, dup := seen[ch]; dup {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	if len(out) == 0 && useDefaultOnEmpty {
		out = make([]string, len(DefaultMenuAvailableChannels))
		copy(out, DefaultMenuAvailableChannels)
	}
	return out, nil
}

// NormalizeMenuChannelPrices keeps prices only for selected channels.
// Missing prices default to basePrice. Rejects unknown channels and negative prices.
func NormalizeMenuChannelPrices(
	channels []string,
	prices map[string]float64,
	basePrice float64,
) (map[string]float64, error) {
	out := make(map[string]float64, len(channels))
	for _, ch := range channels {
		if _, ok := allowedMenuChannels[ch]; !ok {
			return nil, fmt.Errorf("invalid channel_prices key: %s", ch)
		}
		if prices != nil {
			if p, exists := prices[ch]; exists {
				if p < 0 {
					return nil, fmt.Errorf("channel_prices[%s] must be >= 0", ch)
				}
				out[ch] = p
				continue
			}
		}
		if basePrice < 0 {
			return nil, fmt.Errorf("base price must be >= 0")
		}
		out[ch] = basePrice
	}
	return out, nil
}
