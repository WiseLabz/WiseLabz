package connectortest

const piholeV6PageHTML = `<!DOCTYPE html><html><head><title>Pi-hole pihole</title></head><body class="hold-transition"><span class="logo-lg">Pi-<b>hole</b></span></body></html>`

func init() {
	RegisterDiscoveryFixtures("pihole",
		DiscoveryFixture{
			Name: "v5 header only on https", Port: 443, Path: "/admin/", Match: true,
			Response: Response(302, "", "X-Pi-hole", "The Pi-hole Web interface is working!", "Location", "login.php"),
		},
		DiscoveryFixture{
			Name: "v5 header only on http", Port: 80, Path: "/admin/", Match: true,
			Response: Response(302, "", "X-Pi-hole", "The Pi-hole Web interface is working!", "Location", "login.php"),
		},
		DiscoveryFixture{
			Name: "v6 admin page on https", Port: 443, Path: "/admin/", Match: true,
			Response: Response(200, piholeV6PageHTML, "Content-Type", "text/html"),
		},
		DiscoveryFixture{
			Name: "v6 admin page on http", Port: 80, Path: "/admin/", Match: true,
			Response: Response(200, piholeV6PageHTML, "Content-Type", "text/html"),
		},
		DiscoveryFixture{
			Name: "http redirects to https", Port: 80, Path: "/admin/", Match: false,
			Response: Response(302, "", "Location", "https://10.0.0.1/admin/"),
		},
		DiscoveryFixture{
			Name: "generic login page", Port: 443, Path: "/admin/", Match: false,
			Response: Response(200, `<html><head><title>Login</title></head><body><form><input name="username"><input name="password" type="password"></form></body></html>`),
		},
		DiscoveryFixture{
			Name: "pfsense login page", Port: 443, Path: "/admin/", Match: false,
			Response: Response(200, pfsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "opnsense login page", Port: 443, Path: "/admin/", Match: false,
			Response: Response(200, opnsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "truenas web ui", Port: 443, Path: "/admin/", Match: false,
			Response: Response(200, truenasShellHTML, "Content-Type", "text/html"),
		},
	)
}
