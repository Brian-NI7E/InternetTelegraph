# The One-Button Internet Telegraph

### Updates made by Explorer Post 599
11-Nov-2017
- The client has been rewritten to reduce latencies between stations.  When operation multiple clients in the same room, the time differences between them created a cancofony of sounds.
- Modified the server reconnect to silently reconnect if it could be done quickly.  Longer disconnects and reconnects will still notifiy the user.
-- short server disconnects and reconnects happen silently
-- 8 dits signify long server disconnects
-- 2 dits signify the server has reconnected after a long disconnect
- Reorganized the code and added channels for interprocess communications

TODO
- Allow mixing of multiple senders.  On the radio, multiple people might start sending at the same time.  By counting the 'keydown' and 'keyup' events, the tone should play until all senders stop sending.
- Add timeout to prevent infinite 'keydown'.  For example if someone start sending a tone, then loses their connection to the server, all client would continue to play a tone.
-- currently pressing an releasing your telegraph key should stop the tone locally.
- Allow Morse code playback speed to be set in the json file.
- Allow interruption of code playback by pressing the key.
- Organize the main loop as a scheduler based on frequency tasks need to run
- 

## Original REAME.md by Autodidacts
The easiest way to install the internet telegraph client is to use our pre-built SD card image: just download it from the [releases page](https://github.com/TheAutodidacts/InternetTelegraph/releases) and follow the installation instructions in the build tutorial.

But some of you may way want to tinker with the code, or have already have Raspbian installed and configured, and want to run the telegraph from within your existing installation. If that sounds like you, read on!

### Building the client from source

Install Golang for your platform. On Linux, you can do that with:

```
sudo apt install golang
```

Set the relevant environment variables with:

```
export GOOS=linux GOARCH=arm
```

And build with:

```
go build -o internet-telegraph client.go
```

### Installing the telegraph software

First, install Raspbian by following Raspberry Pi’s [official installation instructions](https://www.raspberrypi.org/documentation/installation/).

There are two ways to go from here: you can take the SD card out of your Pi and add the necessary files manually, or you can do the entire thing over SSH.

**To install it on the SD card directly:**

1. Take the SD card out of your Pi and plug it into your SD card reader
- [Download the telegraph code](https://github.com/TheAutodidacts/InternetTelegraph/archive/master.zip) from GitHub.
- Build the telegraph client as described above, or download the prebuilt binary from the ["releases" page](https://github.com/TheAutodidacts/InternetTelegraph/releases/latest).
- Drag the internet telegraph binary (`internet-telegraph`) and the internet telegraph configuration file (`config.json`) into the root directory of your Pi.
- Drag `rc.local` into the `/etc` directory, and replace the rc.local that is already there. (If you’ve customized your rc.local for other reasons, copy the relevant portions into your rc.local rather than overwriting it.)
- Set up your Pi to connect to your wifi network by following the [official instructions on raspberrypi.org](https://www.raspberrypi.org/documentation/configuration/wireless/wireless-cli.md)
- Eject your SD card, pop it back into your Pi, and boot it up!

**To install the internet telegraph client over SSH:**

1. Drop a file called `ssh` (such as the one in this repository) into the `boot` partition of your SD card to enable SSH access.
- Boot up your Pi and connect it to the internet
- SSH into your Pi. You can use [nmap](http://nmap.org) (`nmap 192.168.1.0/24`) to find your Pi’s IP address, or try `ssh pi@raspberrypi.local`.
- Type in your Pi’s password (which you have hopefully changed from the default, "raspberry")
- [Download the telegraph code](https://github.com/TheAutodidacts/InternetTelegraph/archive/master.zip) from GitHub.
- Build the telegraph client for ARM, as described above, or download the prebuilt binary from the ["releases" page]https://github.com/TheAutodidacts/InternetTelegraph/releases/latest) and stick it in the same directory as the sourcecode.
- Copy the files to your Pi over SSH with the following three commands:

```
cd ~/Downloads/internet-telegraph-master/

scp internet-telegraph config.json pi@raspberrypi.local:/

scp rc.local pi@raspberrypi.local:/etc
```

 (replacing `~/Downloads/internet-telegraph-master` with the path to your local copy of the internet telegraph code).
- Test it manually with `./internet-telegraph`
- Reboot your Pi with `sudo reboot`
