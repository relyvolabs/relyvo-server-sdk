// Package relyvo is the official Go server SDK for the Relyvo platform.
//
// It exists to exchange an API key for the short-lived credentials a browser
// needs: a LiveKit participant token and a chat end-user token. The key is a
// server credential and is never sent to a client; only the minted tokens are.
//
//	client, err := relyvo.New(relyvo.Config{
//		APIKey:  os.Getenv("RELYVO_API_KEY"),
//		APIURL:  "https://relyvo.example.com/api",
//		ChatURL: "https://chat-api.example.com",
//	})
//	if err != nil {
//		return err
//	}
//
//	video, err := client.Video.CreateToken(ctx, relyvo.VideoTokenInput{
//		RoomName:            "support-42",
//		ParticipantIdentity: "user_7",
//	})
package relyvo

// Version is the SDK release. It is part of the User-Agent so a service can
// attribute traffic to a client version without asking.
const Version = "0.1.0"

// userAgent is what every request sends.
const userAgent = "relyvo-server-sdk-go/" + Version
