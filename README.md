This tool adds zero-value return values to incomplete Go return
statements, to save you time when writing Go. It is inspired by
and based on goimports.

![short screencast](screencast.gif)

full 30-second screencast: http://youtu.be/hyEMO9vtKZ8

For example, the following incomplete return statement:

	func F() (*MyType, int, error) { return errors.New("foo") }

is made complete by adding nil and 0 returns (the zero values for
*MyType and int):

	func F() (*MyType, int, error) { return nil, 0, errors.New("foo") }

To install via **go get**:

	go get -u github.com/sqs/goreturns

To install from **source**:

1. Clone this repo:

```
git clone https://github.com/sqs/goreturns
```

2. Modifiy the line $23$ from:

```
"github.com/sqs/goreturns/returns"
```

​	to:

```
"./returns"
```

3. Compile source by building:

```
go build goreturns.go
```

4. A binary name `goreturns` will generated in the source directory. Move the binary to your go directory, such as in Mac, `~/go`.

To run:

	goreturns file.go

To view a diff showing what it'd do on a sample file:

	goreturns -d $GOPATH/github.com/sqs/goreturns/_sample/a.go

Editor integration: replace gofmt or goimports in your post-save hook
with goreturns. By default goreturns calls goimports on files before
performing its own processing.

It acts the same as gofmt (same flags, etc) but in addition to code
formatting, also fixes returns.
