Use Amazon alexa with a simple: `alexa ask`

Requirements:
* Portaudio: `brew install portaudio`
* MPG123: `brew install mpg123`

Then `go get github.com/evanphx/alexa/...`

And you'll have `alexa` in `$GOPATH/bin`.

You'll need to sign up and create a developer device with amazon. Here are the instructions lovely copied from AlexaPi:

Next you need to obtain a set of credentials from Amazon to use the Alexa Voice service, login at http://developer.amazon.com and Goto Alexa then Alexa Voice Service You need to create a new product type as a Device, for the ID use something like AlexaPi, create a new security profile and under the web settings allowed origins put http://localhost:5000 and as a return URL put http://localhost:5000/code you can also create URLs replacing localhost with the IP of your Pi eg http://192.168.1.123:5000 Make a note of these credentials you will be asked for them during the install process

Then run `alexa setup`. The values you'll need from creating it are:

* Device Type ID into `--product-id`
* Client ID into `--id`
* Client Secret int `--secret`

Enjoy!
