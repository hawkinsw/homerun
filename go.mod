module github.com/hawkinsw/homerun/v2

go 1.18

//replace golang.org/x/net => /home/hawkinsw/code/gosrc/net

require (
	golang.org/x/net v0.0.0-20220812174116-3211cb980234
	golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab
)

require golang.org/x/text v0.3.7 // indirect
