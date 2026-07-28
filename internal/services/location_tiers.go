package services

import "strings"

const (
	CityTier1 = "tier_1"
	CityTier2 = "tier_2"
	CityTier3 = "tier_3"
)

type DistrictTierOption struct {
	Name string `json:"name"`
	Tier string `json:"tier"`
}

type StateDistrictOptions struct {
	State     string               `json:"state"`
	Districts []DistrictTierOption `json:"districts"`
}

var indiaLocationOptions = []StateDistrictOptions{
	stateOptions("Andhra Pradesh", tier2("Visakhapatnam", "Vijayawada", "Guntur"), tier3()),
	stateOptions("Arunachal Pradesh", tier3()),
	stateOptions("Assam", tier2("Kamrup Metropolitan", "Dibrugarh", "Silchar"), tier3()),
	stateOptions("Bihar", tier2("Patna", "Muzaffarpur", "Gaya"), tier3()),
	stateOptions("Chhattisgarh", tier2("Raipur", "Bilaspur", "Durg"), tier3()),
	stateOptions("Goa", tier2("North Goa", "South Goa"), tier3()),
	stateOptions("Gujarat", tier1("Ahmedabad", "Surat", "Vadodara"), tier2("Rajkot", "Gandhinagar", "Bhavnagar"), tier3()),
	stateOptions("Haryana", tier2("Gurugram", "Faridabad", "Panipat", "Hisar"), tier3()),
	stateOptions("Himachal Pradesh", tier3()),
	stateOptions("Jharkhand", tier2("Ranchi", "Jamshedpur", "Dhanbad"), tier3()),
	stateOptions("Karnataka", tier1("Bengaluru Urban"), tier2("Mysuru", "Mangaluru", "Hubballi-Dharwad", "Belagavi"), tier3()),
	stateOptions("Kerala", tier2("Thiruvananthapuram", "Ernakulam", "Kozhikode", "Thrissur"), tier3()),
	stateOptions("Madhya Pradesh", tier2("Indore", "Bhopal", "Jabalpur", "Gwalior"), tier3()),
	stateOptions("Maharashtra", tier1("Mumbai City", "Mumbai Suburban", "Pune"), tier2("Nagpur", "Nashik", "Thane", "Aurangabad"), tier3()),
	stateOptions("Manipur", tier3()),
	stateOptions("Meghalaya", tier3()),
	stateOptions("Mizoram", tier3()),
	stateOptions("Nagaland", tier3()),
	stateOptions("Odisha", tier2("Khordha", "Cuttack", "Sundargarh"), tier3()),
	stateOptions("Punjab", tier2("Ludhiana", "Amritsar", "Jalandhar", "Patiala", "SAS Nagar"), tier3()),
	stateOptions("Rajasthan", tier2("Jaipur", "Jodhpur", "Udaipur", "Kota"), tier3()),
	stateOptions("Sikkim", tier3()),
	stateOptions("Tamil Nadu", tier1("Chennai"), tier2("Coimbatore", "Madurai", "Tiruchirappalli", "Salem"), tier3()),
	stateOptions("Telangana", tier1("Hyderabad"), tier2("Warangal", "Ranga Reddy", "Karimnagar"), tier3()),
	stateOptions("Tripura", tier3()),
	stateOptions("Uttar Pradesh", tier2("Lucknow", "Kanpur Nagar", "Ghaziabad", "Noida", "Varanasi", "Agra", "Prayagraj"), tier3()),
	stateOptions("Uttarakhand", tier2("Dehradun", "Haridwar"), tier3()),
	stateOptions("West Bengal", tier1("Kolkata"), tier2("Howrah", "Darjeeling", "Siliguri", "Durgapur"), tier3()),
	stateOptions("Andaman and Nicobar Islands", tier3()),
	stateOptions("Chandigarh", tier2("Chandigarh"), tier3()),
	stateOptions("Dadra and Nagar Haveli and Daman and Diu", tier3()),
	stateOptions("Delhi", tier1("New Delhi", "Central Delhi", "South Delhi", "West Delhi", "North West Delhi"), tier2("South West Delhi", "East Delhi", "North Delhi"), tier3()),
	stateOptions("Jammu and Kashmir", tier2("Srinagar", "Jammu"), tier3()),
	stateOptions("Ladakh", tier3()),
	stateOptions("Lakshadweep", tier3()),
	stateOptions("Puducherry", tier2("Puducherry"), tier3()),
}

func stateOptions(state string, groups ...[]DistrictTierOption) StateDistrictOptions {
	districts := make([]DistrictTierOption, 0)
	for _, group := range groups {
		districts = append(districts, group...)
	}
	return StateDistrictOptions{State: state, Districts: districts}
}

func tier1(names ...string) []DistrictTierOption { return tierOptions(CityTier1, names...) }
func tier2(names ...string) []DistrictTierOption { return tierOptions(CityTier2, names...) }
func tier3() []DistrictTierOption               { return tierOptions(CityTier3, "Other district") }

func tierOptions(tier string, names ...string) []DistrictTierOption {
	out := make([]DistrictTierOption, 0, len(names))
	for _, name := range names {
		out = append(out, DistrictTierOption{Name: name, Tier: tier})
	}
	return out
}

func IndiaLocationOptions() []StateDistrictOptions {
	out := make([]StateDistrictOptions, len(indiaLocationOptions))
	copy(out, indiaLocationOptions)
	return out
}

func ResolveCityTier(state, district string) (string, bool) {
	stateKey := normalizeLocationKey(state)
	districtKey := normalizeLocationKey(district)
	if stateKey == "" || districtKey == "" {
		return "", false
	}
	for _, option := range indiaLocationOptions {
		if normalizeLocationKey(option.State) != stateKey {
			continue
		}
		for _, districtOption := range option.Districts {
			if normalizeLocationKey(districtOption.Name) == districtKey {
				return districtOption.Tier, true
			}
		}
		return "", false
	}
	return "", false
}

func NormalizeTierLabel(tier string) string {
	switch strings.TrimSpace(strings.ToLower(tier)) {
	case CityTier1:
		return CityTier1
	case CityTier2:
		return CityTier2
	case CityTier3:
		return CityTier3
	default:
		return CityTier3
	}
}

func normalizeLocationKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
