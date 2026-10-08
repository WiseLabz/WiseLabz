package connectortest

func init() {
	RegisterDiscoveryFixtures("adguardhome",
		DiscoveryFixture{
			Name: "status without login", Port: 3000, Path: "/control/status", Match: true,
			Response: Response(200, `{"dns_port":53,"running":true,"protection_enabled":true,"version":"v0.107.52"}`,
				"Server", "AdGuardHome/v0.107.52", "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "login enabled, bare 401 (not recognised)", Port: 3000, Path: "/control/status", Match: false,
			Response: Response(401, ""),
		},
		DiscoveryFixture{
			Name: "same json from a proxy", Port: 3000, Path: "/control/status", Match: false,
			Response: Response(200, `{"dns_port":53,"running":true,"protection_enabled":true,"version":"v0.107.52"}`,
				"Server", "nginx/1.25.3", "Content-Type", "application/json"),
		},
	)
}
