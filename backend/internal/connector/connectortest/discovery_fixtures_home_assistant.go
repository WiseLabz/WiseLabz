package connectortest

func init() {
	RegisterDiscoveryFixtures("home_assistant",
		DiscoveryFixture{
			Name: "web app manifest", Port: 8123, Path: "/manifest.json", Match: true,
			Response: Response(200, `{"background_color":"#FFFFFF","description":"Home automation platform that puts local control and privacy first.","dir":"ltr","display":"standalone","lang":"en-US","name":"Home Assistant","short_name":"Home Assistant","start_url":"/?homescreen=1","id":"/?homescreen=1"}`,
				"Content-Type", "application/manifest+json"),
		},
		DiscoveryFixture{
			Name: "other pwa manifest", Port: 8123, Path: "/manifest.json", Match: false,
			Response: Response(200, `{"name":"Grafana","short_name":"Grafana"}`, "Content-Type", "application/manifest+json"),
		},
		DiscoveryFixture{
			Name: "not found", Port: 8123, Path: "/manifest.json", Match: false,
			Response: Response(404, `{"message":"404: Not Found"}`, "Content-Type", "application/json"),
		},
	)
}
