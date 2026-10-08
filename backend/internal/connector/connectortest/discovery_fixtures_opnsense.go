package connectortest

const opnsenseLoginHTML = `<!doctype html>
<html lang="en" class="no-js">
  <head>
    <meta name="robots" content="noindex, nofollow" />
    <title>Login | OPNsense</title>
    <link href="/ui/themes/opnsense/build/css/main.css?v=0a1b2c" rel="stylesheet">
    <script src="/ui/js/jquery-3.5.1.min.js"></script>
  </head>
  <body class="page-login">
    <form class="clearfix" id="iform" name="iform" method="post" autocomplete="off">
      <label for="usernamefld">Username:</label>
      <input id="usernamefld" type="text" name="usernamefld" class="form-control user" />
      <input id="passwordfld" type="password" name="passwordfld" class="form-control pwd" />
    </form>
`

func init() {
	RegisterDiscoveryFixtures("opnsense",
		DiscoveryFixture{
			Name: "login page on https", Port: 443, Path: "/", Match: true,
			Response: Response(200, opnsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "login page on http", Port: 80, Path: "/", Match: true,
			Response: Response(200, opnsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "http redirects to https", Port: 80, Path: "/", Match: false,
			Response: Response(302, "", "Location", "https://10.0.0.1/"),
		},
		DiscoveryFixture{
			Name: "pfsense login page", Port: 443, Path: "/", Match: false,
			Response: Response(200, pfsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "truenas web ui", Port: 443, Path: "/", Match: false,
			Response: Response(200, truenasShellHTML, "Content-Type", "text/html"),
		},
		DiscoveryFixture{
			Name: "pi-hole admin page", Port: 443, Path: "/", Match: false,
			Response: Response(200, piholeV6PageHTML, "Content-Type", "text/html"),
		},
	)
}
