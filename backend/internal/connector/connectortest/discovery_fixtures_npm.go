package connectortest

func init() {
	RegisterDiscoveryFixtures("npm",
		DiscoveryFixture{
			Name: "api root 2.13.0 with setup flag", Port: 81, Path: "/api/", Match: true,
			Response: Response(200, `{"status":"OK","setup":true,"version":{"major":2,"minor":13,"revision":0}}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "api root before 2.13 has no setup flag", Port: 81, Path: "/api/", Match: true,
			Response: Response(200, `{"status":"OK","version":{"major":2,"minor":12,"revision":6}}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "version as a string", Port: 81, Path: "/api/", Match: false,
			Response: Response(200, `{"status":"OK","version":"2.12.6"}`, "Content-Type", "application/json"),
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
