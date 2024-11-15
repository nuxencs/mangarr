# Features

- Download from multiple different sources
- Customizable chapter naming
- Automatically download any new chapters

# Examples

```bash
# Download the latest chapter of Jujutsu Kaisen from TCB Scans
mangarr download -d ./downloads -s "tcbscans" -m "Jujutsu Kaisen" -L

# Download the first english chapter of Berserk released by Evil Genius from MangaDex
mangarr download -d ./downloads -s "mangadex" -1 -m "801513ba-a712-498c-8f57-cae55b38cc92" -g "277df5c9-a486-40f6-8dfa-c086c6b60935" -l "en"

# Download chapter 6 and 17 of Chainsaw Man from MANGA Plus
mangarr download -d ./downloads -s "mangaplus" -m "100037" -C "6,17"

# Download the latest chapter of Solo Leveling: Ragnarok from Flame Comics
mangarr download -d ./downloads -s "flamecomics" -m "https://flamecomics.xyz/series/solo-leveling-ragnarok/"

# Download the latest chapter of Solo Max-Level Newbie from Asura Scans
mangarr download -d ./downloads -s "asurascans" -m "https://asuracomic.net/series/solo-max-level-newbie-31f980f5"

# Download chapter 1-3 of One Punch Man from Cubari
mangarr download -d ./downloads -s "cubari" -m "https://git.io/OPM" -g "/r/OnePunchMan" -C "1-3"

# Start monitoring all the manga in your config
mangarr monitor -c ./config/mangarr
```