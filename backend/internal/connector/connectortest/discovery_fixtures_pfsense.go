package connectortest

const pfsenseLoginHTML = `<!DOCTYPE html>
<html lang="en">
	<head>
		<link rel="stylesheet" href="/vendor/bootstrap/css/bootstrap.min.css" type="text/css">
		<link rel="stylesheet" href="/css/login.css?v=1696523114" type="text/css">
		<title>pfSense - Login</title>
	</head>
	<body id="login" >
		<div id="total">
			<h4>Login to pfSense</h4>
			<form method="post" autocomplete="off" class="login">
				<p class="form-title">Sign In</p>
				<input name="usernamefld" id="usernamefld" type="text" placeholder="Username" />
				<input name="passwordfld" id="passwordfld" type="password" placeholder="Password" />
				<input type="submit" name="login" value="Sign In" class="btn btn-success btn-sm" />
			</form>
`

func init() {
	RegisterDiscoveryFixtures("pfsense",
		DiscoveryFixture{
			Name: "login page on https", Port: 443, Path: "/", Match: true,
			Response: Response(200, pfsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "login page on http", Port: 80, Path: "/", Match: true,
			Response: Response(200, pfsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "http redirects to https", Port: 80, Path: "/", Match: false,
			Response: Response(302, "", "Location", "https://10.0.0.1/"),
		},
		DiscoveryFixture{
			Name: "opnsense login page", Port: 443, Path: "/", Match: false,
			Response: Response(200, opnsenseLoginHTML, "Content-Type", "text/html; charset=UTF-8"),
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
