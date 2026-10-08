package connectortest

const truenasShellHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title id="main-page-title"></title>
  <base href="/">
  <link rel="manifest" href="assets/favicons/site.webmanifest">
</head>
<body class="ix-dark">
  <ix-root>
    <div style="display: flex; align-items: center; justify-content: center; height: 100vh;"></div>
  </ix-root>
</body>
</html>
`

func init() {
	RegisterDiscoveryFixtures("truenas",
		DiscoveryFixture{
			Name: "web ui shell on https", Port: 443, Path: "/ui/", Match: true,
			Response: Response(200, truenasShellHTML, "Content-Type", "text/html"),
		},
		DiscoveryFixture{
			Name: "web ui shell on http", Port: 80, Path: "/ui/", Match: true,
			Response: Response(200, truenasShellHTML, "Content-Type", "text/html"),
		},
		DiscoveryFixture{
			Name: "root redirect to ui without a body", Port: 80, Path: "/ui/", Match: false,
			Response: Response(302, "", "Location", "http://10.0.0.1/ui/"),
		},
		DiscoveryFixture{
			Name: "http redirects to https", Port: 80, Path: "/ui/", Match: false,
			Response: Response(302, "", "Location", "https://10.0.0.1/"),
		},
		DiscoveryFixture{
			Name: "pfsense login page", Port: 443, Path: "/ui/", Match: false,
			Response: Response(200, pfsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "opnsense login page", Port: 443, Path: "/ui/", Match: false,
			Response: Response(200, opnsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "pi-hole admin page", Port: 443, Path: "/ui/", Match: false,
			Response: Response(200, piholeV6PageHTML, "Content-Type", "text/html"),
		},
	)
}
