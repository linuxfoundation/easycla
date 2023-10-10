package main

import (
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/v2/docusign_auth"
)

func main() {
	integrationKey := "557677f2-1e2f-4955-aaa6-1ef44630e01d"
	userID := "3a1d118f-3083-4c25-8306-11b7400f7c03"
	privateKey :=`
	-----BEGIN RSA PRIVATE KEY-----
	MIIEogIBAAKCAQEAh0M2mIGaJjP8S/FxZR7nRsatCR/KpCPBFBbxalZffykqtTID
	KNeDhJ5RvJKAVoJlLaLoUYSYloVaeSAwQdbn4F+Lsnll3mCGocwdl/W8998Lc/Ln
	MaNQhpekBoXaq8vbj251jxnRcsdI9yl/YyQo3jZnM77OWtEF7dyvS6V9cMprT2Ca
	eIJPj0Ck3/P9rGwE3DdiEXZDetgkuNQyMavfvLKltCNu3qQXFA95PXlHs2E57OrS
	CNIQT37jfInXuIyCoGjDSq+U3ZE047UG5Id8/OFvcP1z5iUdwvueoEximt/kSvR0
	ZcNfjqyJnnYq80OKwTdalZEOzAEkt/U2QuB9jwIDAQABAoIBAB01GstopOgd7ptZ
	efJrb2ZdjUy8lC3IWK9lWuDq4LkdIw84Su1dSBVxeFXfTp4fjwiBNmgv2SEbj5M7
	K6Bz7uMIzqoNw7z2m+vBHxzKn/DoNVlmuJyD1uYRRYZxDext2y3IHNN3MD54IN3a
	FJtMWhTNq5BFYdrDauPXdPTBOeqKQgMoQl6OwHIx4WJXYxRgIgrvBYRzLSwG/u4V
	tAvj1/J0PSSOY5A1qn6L2ii9abVElFr2zZC5MtG3oG1TLCidzxZFt/5qSHGo+0tU
	l/7YRXWxadfYrPmS9akbC7SQn9WJQdIlfejtuQ6hmKTCCjhTp2BUba3kTI3PcM46
	GzW3WQECgYEA08lwWFZng3bm3KrDx3ljjZ+cuL8vOXfLeYuaIPmNu5eXrR5ZR9Ar
	UyUmnZbL8rhK9dNCmRGX293uuppwJolfMAy+kcvQ4HsRQ5tOgXmP8VhKzA4RRc+h
	I7uIGQ7NmKI6Wm/wJ+T3Okw0h8ze8MW66/D5IHDQSoGQZH9kkA9+MQECgYEAo4AS
	jB/b/SWhwrRUI8m+5Tjy5WO9ZkRZ54zpRnAr5HsjSBgb72IGbJKwleKwb3FxwPTQ
	ZlgKP2O5rGzZOEZcIryIliCpV4C/XeFVwUKxo/A5jkL+U9ZQwoA0/0ssdE/7uLzG
	FiKcFK7OhsFg+dNEX1ForM9cGN5OzY8sGzylHo8CgYAc0Nq1WkRJUeNFgQKUYILY
	ITB8vp6ZTiBkUEdPV0Uekhi0GF4DdGKAtJxVctAbHVItsmnsU8V6x+6UezDpPWWz
	Lvi686Ve9b+6mCYNXdHk/6NlskBNZFvDdd+lsSruKpyP840UkIXG69l15L0su2qc
	cbQj4tWkXY6c7exr4X/FAQKBgDmGTABFDU9puBoa/CeDSci4Wq1ehDrA/ai8KS8B
	NFA1CtrIsLtuj7gPfFWf5levYEh1WgVIIILhAWiq+1oTV0NZdezsHOiOgcX0DAns
	/zcgw/9LjtPMaamlFgBkYIWjxnre4ArVrniQcFV1IDuFm16189ApPMv7G1qzbt8+
	XRH9AoGAceSrZLbceDkqgprTTV1BEOFY+Ti9edBIUW+CqN6KkB66OXHGWlab/PZc
	OCHRPAjEijZcVXm8IPC2yi/s0agQAB8dKO2L1X0EtvxkWSg2s/YXFpp0QccQToTo
	lRFb9injNzixUOq18Z62XcC/yqMB7QtgRw5x5OQk0HNWpL7h6KM=
	-----END RSA PRIVATE KEY-----
	`
	token, err := docusignauth.GetAccessToken(integrationKey, userID, privateKey)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(token)
}