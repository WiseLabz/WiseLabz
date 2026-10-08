package connectortest

func init() {
	RegisterDiscoveryFixtures("npm",
		DiscoveryFixture{
			Name: "api root", Port: 81, Path: "/api/", Match: true,
			Response: Response(200, `{"status":"OK","setup":true,"version":{"major":2,"minor":12,"revision":3}}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "status only", Port: 81, Path: "/api/", Match: false,
			Response: Response(200, `{"status":"OK"}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "traefik-style version", Port: 81, Path: "/api/", Match: false,
			Response: Response(200, `{"Version":"3.1.2","Codename":"comte","startDate":"2024-09-10T10:00:00Z"}`, "Content-Type", "application/json"),
		},
	)
}
