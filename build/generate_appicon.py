"""Render the monochrome reminder app icon at high resolution for Wails."""

from pathlib import Path

from PIL import Image, ImageDraw


SIZE = 1024
SCALE = 3
WHITE = (255, 255, 255, 255)
DARK = (16, 17, 22, 255)
image = Image.new("RGBA", (SIZE * SCALE, SIZE * SCALE), (0, 0, 0, 0))
draw = ImageDraw.Draw(image)


def box(cx, cy, radius):
    x, y = round(cx * SCALE), round(cy * SCALE)
    r = round(radius * SCALE)
    return x - r, y - r, x + r, y + r


def stroke(points, width):
    scaled = [(round(x * SCALE), round(y * SCALE)) for x, y in points]
    line_width = round(width * SCALE)
    draw.line(scaled, fill=WHITE, width=line_width, joint="curve")
    radius = line_width // 2
    for x, y in (scaled[0], scaled[-1]):
        draw.ellipse((x - radius, y - radius, x + radius, y + radius), fill=WHITE)


draw.rounded_rectangle(
    (40 * SCALE, 40 * SCALE, 984 * SCALE, 984 * SCALE),
    radius=230 * SCALE,
    fill=DARK,
)

# Minimal clock face and two hands set at 10:10.
draw.ellipse(box(512, 512, 320), outline=WHITE, width=56 * SCALE)
stroke([(512, 512), (414, 378)], 52)
stroke([(512, 512), (655, 375)], 52)
draw.ellipse(box(512, 512, 32), fill=WHITE)

# A small notification badge sits over the top-right edge of the clock.
draw.ellipse(box(756, 268, 47), fill=DARK)
draw.ellipse(box(756, 268, 31), fill=WHITE)

image.resize((SIZE, SIZE), Image.Resampling.LANCZOS).save(
    Path(__file__).with_name("appicon.png"), optimize=True
)
