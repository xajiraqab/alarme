"""Generate the short, local reminder chimes used by the frontend."""

import math
import struct
import wave
from pathlib import Path


RATE = 22050
OUTPUT = Path(__file__).parent / "public" / "sounds"


def bell_sample(t, frequency, partials, decay):
    attack = min(1.0, t / 0.012)
    return attack * sum(
        amplitude * math.sin(2 * math.pi * frequency * ratio * t)
        * math.exp(-(2.1 + ratio * 0.48) * t)
        for ratio, amplitude in partials
    ) * math.exp(-decay * t)


def write_chime(filename, notes, duration):
    frames = []
    count = round(RATE * duration)
    for index in range(count):
        t = index / RATE
        sample = 0.0
        for start, frequency, note_partials, decay, gain in notes:
            local_t = t - start
            if local_t >= 0:
                sample += gain * bell_sample(local_t, frequency, note_partials, decay)
        # A quiet fade on the final 45 ms prevents a hard edge.
        fade = min(1.0, (duration - t) / 0.045)
        value = max(-1.0, min(1.0, sample * 0.42 * fade))
        frames.append(struct.pack("<h", round(value * 32767)))

    OUTPUT.mkdir(parents=True, exist_ok=True)
    with wave.open(str(OUTPUT / filename), "wb") as audio:
        audio.setnchannels(1)
        audio.setsampwidth(2)
        audio.setframerate(RATE)
        audio.writeframes(b"".join(frames))


harmonic = [(1.0, 1.0), (2.01, 0.38), (3.98, 0.16), (5.43, 0.07)]
glass = [(1.0, 1.0), (2.756, 0.32), (5.404, 0.14), (8.933, 0.06)]
warm = [(1.0, 1.0), (2.0, 0.27), (3.01, 0.12), (4.0, 0.045)]

write_chime(
    "soft-chime.wav",
    [(0.0, 783.99, harmonic, 2.3, 0.8), (0.24, 1174.66, harmonic, 2.7, 0.62)],
    1.35,
)
write_chime(
    "glass-bells.wav",
    [(0.0, 783.99, glass, 4.5, 0.78), (0.17, 987.77, glass, 4.7, 0.66),
     (0.34, 1174.66, glass, 4.9, 0.58)],
    1.25,
)
write_chime(
    "warm-tone.wav",
    [(0.0, 523.25, warm, 1.7, 0.82), (0.28, 783.99, warm, 2.1, 0.62)],
    1.6,
)
write_chime(
    "dewdrop.wav",
    [(0.0, 1046.5, glass, 5.0, 0.72), (0.14, 1318.51, glass, 5.2, 0.62),
     (0.31, 1567.98, glass, 5.4, 0.52)],
    1.18,
)
write_chime(
    "aurora.wav",
    [(0.0, 392.0, harmonic, 2.3, 0.56), (0.15, 523.25, harmonic, 2.4, 0.56),
     (0.31, 659.25, harmonic, 2.5, 0.52), (0.49, 783.99, harmonic, 2.6, 0.48)],
    1.72,
)
write_chime(
    "pulse.wav",
    [(0.0, 392.0, warm, 1.7, 0.78), (0.26, 493.88, warm, 1.8, 0.7),
     (0.55, 392.0, warm, 2.0, 0.48)],
    1.48,
)
write_chime(
    "twinkle.wav",
    [(0.0, 880.0, glass, 5.2, 0.66), (0.13, 1318.51, glass, 5.4, 0.6),
     (0.29, 1046.5, glass, 5.5, 0.55), (0.45, 1567.98, glass, 5.7, 0.48)],
    1.28,
)
write_chime(
    "moon-sun.wav",
    [(0.0, 440.0, harmonic, 2.3, 0.62), (0.16, 523.25, harmonic, 2.4, 0.58),
     (0.34, 659.25, harmonic, 2.5, 0.54), (0.56, 587.33, harmonic, 2.7, 0.48)],
    1.55,
)
write_chime(
    "rain-glass.wav",
    [(0.0, 783.99, glass, 4.6, 0.62), (0.2, 1174.66, glass, 4.9, 0.52),
     (0.42, 987.77, glass, 5.1, 0.46)],
    1.4,
)
write_chime(
    "hearth.wav",
    [(0.0, 349.23, warm, 1.7, 0.72), (0.24, 523.25, warm, 1.9, 0.6),
     (0.52, 440.0, warm, 2.1, 0.54)],
    1.7,
)
write_chime(
    "orbit.wav",
    [(0.0, 659.25, glass, 5.0, 0.58), (0.14, 987.77, glass, 5.2, 0.52),
     (0.3, 783.99, glass, 5.4, 0.5), (0.48, 1174.66, glass, 5.6, 0.44)],
    1.35,
)
write_chime(
    "ode-to-joy.wav",
    [(0.00, 329.63, warm, 2.4, 0.78), (0.22, 329.63, warm, 2.4, 0.7),
     (0.44, 349.23, warm, 2.5, 0.68), (0.66, 392.0, warm, 2.6, 0.66)],
    1.35,
)
write_chime(
    "hungarian-march.wav",
    [(0.00, 293.66, warm, 1.8, 0.8), (0.18, 349.23, warm, 1.9, 0.72),
     (0.36, 392.0, warm, 2.0, 0.72), (0.54, 440.0, warm, 2.1, 0.76),
     (0.82, 415.30, warm, 2.0, 0.68)],
    1.55,
)
write_chime(
    "happy-birthday.wav",
    [(0.00, 293.66, glass, 3.5, 0.68), (0.22, 293.66, glass, 3.5, 0.62),
     (0.46, 329.63, glass, 3.6, 0.62), (0.72, 293.66, glass, 3.6, 0.66),
     (1.05, 392.0, glass, 3.8, 0.64), (1.36, 369.99, glass, 3.7, 0.6)],
    1.9,
)
