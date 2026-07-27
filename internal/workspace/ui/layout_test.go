package ui

import (
	appshell "github.com/Yacobolo/leapview/internal/app/shell"
	webpage "github.com/Yacobolo/leapview/internal/platform/web/page"
)

func testLayoutProvider() webpage.Provider {
	return appshell.Provider(appshell.Config{
		Presentation: webpage.Presentation{ProductName: "LeapView", FaviconPath: "/static/favicon.svg"},
	})
}
