package alexa

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/evanphx/alexa/config"
)

type SetupCommand struct {
	Product string `long:"product-id" description:"Alexa product id"`
	ID      string `long:"id" description:"Client ID"`
	Secret  string `long:"secret" description:"Client Secret"`
}

/*
class Start(object):
	def index(self):
		scope="alexa_all"
		sd = json.dumps({
		    "alexa:all": {
		        "productID": ProductID,
		        "productInstanceAttributes": {
		            "deviceSerialNumber": "001"
		        }
		    }
		})
		url = "https://www.amazon.com/ap/oa"
		callback = cherrypy.url()  + "code"
		payload = {"client_id" : Client_ID, "scope" : "alexa:all", "scope_data" : sd, "response_type" : "code", "redirect_uri" : callback }
		req = requests.Request('GET', url, params=payload)
		p = req.prepare()
		raise cherrypy.HTTPRedirect(p.url)
	def code(self, var=None, **params):
		code = urllib.quote(cherrypy.request.params['code'])
		callback = cherrypy.url()
		payload = {"client_id" : Client_ID, "client_secret" : Client_Secret, "code" : code, "grant_type" : "authorization_code", "redirect_uri" : callback }
		url = "https://api.amazon.com/auth/o2/token"
		r = requests.post(url, data = payload)
		resp = r.json()
		line = 'refresh_token = "{}"'.format(resp['refresh_token'])
		with open("creds.py", 'a') as f:
			f.write(line)
		return "Success!, refresh token has been added to your creds file, you may now reboot the Pi <br>{}".format(resp['refresh_token'])
	index.exposed = True
	code.exposed = True

*/

func (s *SetupCommand) handleCode(res http.ResponseWriter, inreq *http.Request) {
	code := inreq.URL.Query().Get("code")

	form := url.Values{}

	form.Add("client_id", s.ID)
	form.Add("client_secret", s.Secret)
	form.Add("code", code)
	form.Add("grant_type", "authorization_code")
	form.Add("redirect_uri", "http://localhost:5000/code")

	req, err := http.NewRequest("POST", "https://api.amazon.com/auth/o2/token", strings.NewReader(form.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	fmt.Printf("=> %s\n", inreq.URL.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	var oauthResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	err = json.NewDecoder(resp.Body).Decode(&oauthResponse)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	cfg.Product = s.Product
	cfg.ClientId = s.ID
	cfg.ClientSecret = s.Secret
	cfg.AccessToken = oauthResponse.AccessToken
	cfg.RefreshToken = oauthResponse.RefreshToken
	cfg.ExpiresAt = time.Now().UTC().Add(time.Duration(oauthResponse.ExpiresIn) * time.Second)

	err = config.WriteConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	res.Write([]byte("Success! We've got the values, you can close this window now"))
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("You can now interact with alexa using `alexa ask`")
		os.Exit(0)
	}()
}

func (s *SetupCommand) setupHandler(res http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/code" {
		s.handleCode(res, req)
		return
	}

	sd := fmt.Sprintf(`
		{
			"alexa:all": {
				"productID": "%s",
				"productInstanceAttributes": {
					"deviceSerialNumber": "001"
		    }
		  }
		}`, s.Product)

	req, err := http.NewRequest("GET", "https://www.amazon.com/ap/oa", nil)
	if err != nil {
		log.Fatal(err)
	}

	u, err := url.Parse("https://www.amazon.com/ap/oa")
	if err != nil {
		log.Fatal(err)
	}

	q := u.Query()
	q.Add("client_id", s.ID)
	q.Add("scope", "alexa:all")
	q.Add("scope_data", sd)
	q.Add("response_type", "code")
	q.Add("redirect_uri", "http://localhost:5000/code")

	u.RawQuery = q.Encode()

	res.Header().Add("Location", u.String())
	res.WriteHeader(302)
}

func (s *SetupCommand) Execute(args []string) error {
	fmt.Printf("Open http://localhost:5000 to continue with setup")
	return http.ListenAndServe(":5000", http.HandlerFunc(s.setupHandler))
}
