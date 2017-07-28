# pluzzkids

## Windows setup

* Install https://golang.org/
* Install ffmpeg https://www.ffmpeg.org/download.html in `C:\bin`
* Compile for windows `$ GOOS=windows go build .`
* Place `pluzzkids.exe` in `C:\bin`
* Copy `config.json` from this repo to `C:\bin\pluzz-config.json` and edit it to whitelist the shows you want.
* Use https://nssm.cc/ (copy into `C:\bin`) to set a service launching this program
    * Path: `C:\bin\pluzzkids.exe`
    * Startup Directory: `C:\bin`
    * Arguments `-config="C:\bin\pluzz-config.json" -dest="Z:\pathWhereYouWantYourFiles" -log="C:\bin\pluzz.log"`
* Start the service and look at the logs to make sure it goes well