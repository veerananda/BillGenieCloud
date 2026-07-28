package services

import "strings"

// City tiers follow Ministry of Finance HRA X/Y/Z classification:
// X = Tier-1, Y = Tier-2, Z = Tier-3 (all other cities/towns).
const (
	CityTier1 = "tier_1"
	CityTier2 = "tier_2"
	CityTier3 = "tier_3"
)

type CityTierOption struct {
	Name string `json:"name"`
	Tier string `json:"tier"`
}

type StateCityOptions struct {
	State  string           `json:"state"`
	Cities []CityTierOption `json:"cities"`
}

// India HRA X/Y cities organised by state/UT. Every state also includes
// "Other city" as Tier-3 (Z) for towns not on the official X/Y lists.
var indiaLocationOptions = []StateCityOptions{
	stateOptions("Andhra Pradesh",
		tier2("Visakhapatnam", "Vijayawada", "Guntur", "Nellore", "Kakinada", "Kurnool", "Rajahmundry"),
		tier3()),
	stateOptions("Arunachal Pradesh", tier3()),
	stateOptions("Assam",
		tier2("Guwahati"),
		tier3()),
	stateOptions("Bihar",
		tier2("Patna"),
		tier3()),
	stateOptions("Chhattisgarh",
		tier2("Raipur", "Bhilai", "Bilaspur", "Durg"),
		tier3()),
	stateOptions("Goa", tier3()),
	stateOptions("Gujarat",
		tier1("Ahmedabad"),
		tier2("Surat", "Vadodara", "Rajkot", "Jamnagar", "Bhavnagar", "Anand", "Nadiad", "Dahod", "Gandhinagar"),
		tier3()),
	stateOptions("Haryana",
		tier2("Faridabad", "Gurugram", "Karnal"),
		tier3()),
	stateOptions("Himachal Pradesh",
		tier2("Shimla", "Hamirpur"),
		tier3()),
	stateOptions("Jharkhand",
		tier2("Jamshedpur", "Dhanbad", "Ranchi", "Bokaro Steel City"),
		tier3()),
	stateOptions("Karnataka",
		tier1("Bengaluru"),
		tier2("Belagavi", "Hubballi-Dharwad", "Mangaluru", "Mysuru", "Kalaburagi", "Ballari", "Vijayapura", "Raichur"),
		tier3()),
	stateOptions("Kerala",
		tier2("Thiruvananthapuram", "Kochi", "Kozhikode", "Thrissur", "Malappuram", "Kannur", "Kollam"),
		tier3()),
	stateOptions("Madhya Pradesh",
		tier2("Indore", "Bhopal", "Jabalpur", "Gwalior", "Ujjain", "Ratlam"),
		tier3()),
	stateOptions("Maharashtra",
		tier1("Mumbai", "Pune"),
		tier2("Nagpur", "Nashik", "Aurangabad", "Solapur", "Amravati", "Kolhapur", "Sangli", "Jalgaon", "Akola", "Nanded", "Dhule", "Bhiwandi", "Dombivli", "Vasai-Virar", "Pimpri-Chinchwad", "Thane", "Navi Mumbai", "Kalyan-Dombivli"),
		tier3()),
	stateOptions("Manipur", tier3()),
	stateOptions("Meghalaya", tier3()),
	stateOptions("Mizoram", tier3()),
	stateOptions("Nagaland", tier3()),
	stateOptions("Odisha",
		tier2("Bhubaneswar", "Cuttack", "Rourkela", "Berhampur"),
		tier3()),
	stateOptions("Punjab",
		tier2("Ludhiana", "Amritsar", "Jalandhar", "Mohali", "Patiala"),
		tier3()),
	stateOptions("Rajasthan",
		tier2("Jaipur", "Jodhpur", "Kota", "Ajmer", "Bikaner", "Udaipur"),
		tier3()),
	stateOptions("Sikkim", tier3()),
	stateOptions("Tamil Nadu",
		tier1("Chennai"),
		tier2("Coimbatore", "Madurai", "Tiruchirappalli", "Salem", "Tirunelveli", "Erode", "Vellore", "Tiruvannamalai", "Thanjavur", "Kumbakonam"),
		tier3()),
	stateOptions("Telangana",
		tier1("Hyderabad"),
		tier2("Warangal", "Karimnagar"),
		tier3()),
	stateOptions("Tripura", tier3()),
	stateOptions("Uttar Pradesh",
		tier2("Lucknow", "Kanpur", "Ghaziabad", "Agra", "Varanasi", "Meerut", "Prayagraj", "Bareilly", "Aligarh", "Moradabad", "Saharanpur", "Gorakhpur", "Noida", "Jhansi", "Mathura"),
		tier3()),
	stateOptions("Uttarakhand",
		tier2("Dehradun"),
		tier3()),
	stateOptions("West Bengal",
		tier1("Kolkata"),
		tier2("Asansol", "Siliguri", "Durgapur", "Bardhaman", "Purulia", "Howrah"),
		tier3()),
	stateOptions("Andaman and Nicobar Islands", tier3()),
	stateOptions("Chandigarh",
		tier2("Chandigarh"),
		tier3()),
	stateOptions("Dadra and Nagar Haveli and Daman and Diu", tier3()),
	stateOptions("Delhi",
		tier1("Delhi"),
		tier3()),
	stateOptions("Jammu and Kashmir",
		tier2("Srinagar", "Jammu"),
		tier3()),
	stateOptions("Ladakh", tier3()),
	stateOptions("Lakshadweep", tier3()),
	stateOptions("Puducherry",
		tier2("Puducherry"),
		tier3()),
}

func stateOptions(state string, groups ...[]CityTierOption) StateCityOptions {
	cities := make([]CityTierOption, 0)
	for _, group := range groups {
		cities = append(cities, group...)
	}
	return StateCityOptions{State: state, Cities: cities}
}

func tier1(names ...string) []CityTierOption { return tierOptions(CityTier1, names...) }
func tier2(names ...string) []CityTierOption { return tierOptions(CityTier2, names...) }
func tier3() []CityTierOption                 { return tierOptions(CityTier3, "Other city") }

func tierOptions(tier string, names ...string) []CityTierOption {
	out := make([]CityTierOption, 0, len(names))
	for _, name := range names {
		out = append(out, CityTierOption{Name: name, Tier: tier})
	}
	return out
}

func IndiaLocationOptions() []StateCityOptions {
	out := make([]StateCityOptions, len(indiaLocationOptions))
	copy(out, indiaLocationOptions)
	return out
}

// ResolveCityTier derives the pricing tier from an official state + city pair.
func ResolveCityTier(state, city string) (string, bool) {
	stateKey := normalizeLocationKey(state)
	cityKey := normalizeLocationKey(city)
	if stateKey == "" || cityKey == "" {
		return "", false
	}
	for _, option := range indiaLocationOptions {
		if normalizeLocationKey(option.State) != stateKey {
			continue
		}
		for _, cityOption := range option.Cities {
			if normalizeLocationKey(cityOption.Name) == cityKey {
				return cityOption.Tier, true
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
