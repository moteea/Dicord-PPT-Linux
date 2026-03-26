# evdev bindings for Go

This package provides Go language bindings to the generic input event
interface in Linux. The *evdev* interface serves the purpose of
passing events generated in the kernel directly to userspace through
character devices that are typically located in `/dev/input/`.

The project is just a fork of https://github.com/gvalkov/golang-evdev
with merges of useful changes that made in other forks. The
development of the original project was stalled and I decided to move
the code to my own repo and support it for my projects.

*My great thanks to [Georgi Valkov](https://github.com/gvalkov) for*
*the development of the library and support of it for the years!*

## Documentation
API docs at https://pkg.go.dev/github.com/grafov/evdev

Check basic usage example: [example_test.go](example_test.go)

The excerpt from libevdev docs below.

*kernel → libevdev → evtest*

For X.Org input modules, the stack would look like this:

*kernel → libevdev → xf86-input-evdev → X server → X client*

For Weston/Wayland, the stack would look like this:

*kernel → libevdev → Weston → Wayland client*

libevdev does not have knowledge of X clients or Wayland clients, it
is too low in the stack.


## License

Under BSD 3-clause license. See [LICENSE.txt](LICENSE.txt).
