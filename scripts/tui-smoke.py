#!/usr/bin/env python3
"""Drive the genie TUI in a real pseudo-terminal and check it starts and exits.

Usage: scripts/tui-smoke.py [path/to/genie]

It launches the Bubble Tea UI (GENIE_TUI=bt), waits for the welcome message
to be drawn, resizes the terminal, presses Ctrl+C on the empty prompt and
checks the process exits cleanly with the terminal restored. No LLM call is
made, so a placeholder API key is enough.
"""
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

BINARY = sys.argv[1] if len(sys.argv) > 1 else "build/genie"
TIMEOUT = 20.0


def set_winsize(fd, rows, cols):
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))


def read_until(fd, needle, deadline, buf):
    while time.time() < deadline:
        r, _, _ = select.select([fd], [], [], 0.2)
        if not r:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break
        if not chunk:
            break
        buf.extend(chunk)
        if needle in buf:
            return True
    return needle in buf


def main():
    env = dict(os.environ)
    env.setdefault("GEMINI_API_KEY", "smoke-test-placeholder")
    env["GENIE_TUI"] = "bt"
    env["TERM"] = "xterm-256color"
    env["COLORTERM"] = "truecolor"

    pid, fd = pty.fork()
    if pid == 0:
        os.execvpe(BINARY, [BINARY], env)

    set_winsize(fd, 30, 100)
    buf = bytearray()
    deadline = time.time() + TIMEOUT
    try:
        if not read_until(fd, b"Type :? for help", deadline, buf):
            fail("welcome message never rendered", buf)

        # Resize while running: the UI must keep drawing.
        set_winsize(fd, 40, 120)
        os.kill(pid, signal.SIGWINCH)
        os.write(fd, b"smoke")
        if not read_until(fd, b"smoke", time.time() + 5, buf):
            fail("typed text did not echo in the prompt", buf)

        # Clear the prompt, then Ctrl+C on the empty prompt quits.
        os.write(fd, b"\x03")
        time.sleep(0.2)
        os.write(fd, b"\x03")

        waited = 0.0
        while waited < 10:
            wpid, status = os.waitpid(pid, os.WNOHANG)
            if wpid == pid:
                break
            read_until(fd, b"\xff\xff", time.time() + 0.1, buf)  # drain
            waited += 0.1
        else:
            os.kill(pid, signal.SIGKILL)
            fail("process did not exit after Ctrl+C", buf)

        code = os.waitstatus_to_exitcode(status)
        if code != 0:
            fail(f"exit code {code}", buf)
        if b"\x1b[?1049l" not in buf:
            fail("alternate screen was not restored on exit", buf)
        if b"panic:" in buf or b"goroutine " in buf:
            fail("panic in output", buf)
    finally:
        try:
            os.close(fd)
        except OSError:
            pass
    print("tui-smoke: ok (started, echoed input, survived resize, exited cleanly)")


def fail(reason, buf):
    tail = buf[-2000:].decode("utf-8", "replace")
    print(f"tui-smoke: FAIL: {reason}\n--- last output ---\n{tail}", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
