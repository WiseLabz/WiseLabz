package connectortest

func init() {
	RegisterDiscoveryFixtures("traefik",
		DiscoveryFixture{
			Name: "version", Port: 8080, Path: "/api/version", Match: true,
			Response: Response(200, `{"Version":"3.1.2","Codename":"comte","startDate":"2024-09-10T10:00:00.123456Z"}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "portainer-style version", Port: 8080, Path: "/api/version", Match: false,
			Response: Response(200, `{"Version":"2.21.4","InstanceID":"299ab403"}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "npm-style status", Port: 8080, Path: "/api/version", Match: false,
			Response: Response(200, `{"status":"OK","setup":true,"version":{"major":2,"minor":12,"revision":3}}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "not found", Port: 8080, Path: "/api/version", Match: false,
			Response: Response(404, "404 page not found\n", "Content-Type", "text/plain; charset=utf-8"),
		},
	)
}
