package brand

type ProductConfig struct {
	ProductName      string
	ShortName        string
	RepositoryName   string
	Description      string
	CookiePrefix     string
	SupportURL       string
	SecurityURL      string
	LogoPath         string
	ThemeColor       string
	BackgroundColor  string
	DefaultLocale    string
	SupportedLocales []string
	DefaultTheme     string
}

func SupportsLocale(locale string) bool {
	for _, supported := range Config.SupportedLocales {
		if locale == supported {
			return true
		}
	}
	return false
}
