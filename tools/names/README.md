# tools/names

`go run ./tools/names` regenerates `mask/words_corpus.go` from the two CSVs
in this directory. `make names` runs it; `make names-check` (wired into
`make check` and into `.github/workflows/ci.yml`'s `docs` job) regenerates
into memory and fails if the committed `mask/words_corpus.go` differs.
Neither target downloads anything — the CSVs below are the only input, and
they are checked in — so both are offline, on a laptop and in CI alike.

**T-0303 only fetches, extracts and generates this corpus.**
`mask/words_corpus.go`'s `censusGivenWords`/`censusSurnameWords` are not read
by any masker yet; see `mask/CLAUDE.md`'s note on why, and tracker T-0304,
which wires them in after T-0302 lands.

## The source

The U.S. Census Bureau's 2020 Census names release:
<https://www.census.gov/topics/population/genealogy/data/2020_names.html>.
Two of its files, fetched 2026-09-22:

| File | URL | sha256 | Columns (as found) |
|---|---|---|---|
| `Names2020_FirstNames_Sex_Top1000.xlsx` | <https://www2.census.gov/topics/genealogy/2020surnames/Names2020_FirstNames_Sex_Top1000.xlsx> | `b7e8a8ea8cf5babe0220aa9f0f19294664a1818fc61acd43efcd3ff356fa9676` | `FIRST NAME`, `RANK`, `FREQUENCY (COUNT)`, `PROPORTION PER 100,000 POPULATION`, `CUMULATIVE PROPORTION`, `MALE`, `FEMALE` |
| `Names2020_LastNames_RaceHispanic_Top1000.xlsx` | <https://www2.census.gov/topics/genealogy/2020surnames/Names2020_LastNames_RaceHispanic_Top1000.xlsx> | `89108b7321fc4656fba390ea665680ac651786d51effc51da54d4b8be3fdbf6b` | `LAST NAME`, `RANK`, `FREQUENCY (COUNT)`, `PROPORTION PER 100,000 POPULATION`, `CUMULATIVE PROPORTION`, `NON-HISPANIC OR LATINO WHITE ALONE`, `NON-HISPANIC OR LATINO BLACK OR AFRICAN AMERICAN ALONE`, `NON-HISPANIC OR LATINO AMERICAN INDIAN AND ALASKA NATIVE ALONE`, `NON-HISPANIC OR LATINO ASIAN AND NATIVE HAWAIIAN AND OTHER PACIFIC ISLANDER ALONE`, `NON-HISPANIC OR LATINO TWO OR MORE RACES`, `HISPANIC OR LATINO ORIGIN` |

Both are works of the U.S. Government (17 U.S.C. § 105) and so are in the
public domain in the United States. The Bureau asks that a use of its data
be cited: <https://www.census.gov/about/policies/citation.html>. This
project's citation: U.S. Census Bureau, 2020 Census, "Frequently Occurring
Surnames from the 2020 Census" / "Frequently Occurring First Names in the
2020 Census by Sex," <https://www.census.gov/topics/population/genealogy/data/2020_names.html>.

**Only `name`, `sex`, `rank` and `count` are ever read.** The surnames
file's six race and Hispanic-origin proportion columns
(`NON-HISPANIC OR LATINO …`, `HISPANIC OR LATINO ORIGIN`) are never read,
copied or embedded by the extraction below, by the CSVs it produces, or by
`tools/names`' generator — this corpus carries no race or ethnicity data at
all.

## What "sex" means for the first-names file

`Names2020_FirstNames_Sex_Top1000.xlsx` is the Bureau's own overall top
1,000 first names (ranked once, by total `FREQUENCY (COUNT)`), with each
name's `MALE` and `FEMALE` counts carried as two more columns — it has no
per-sex rank of its own. The extraction below derives one: for each sex, the
same 1,000 names are sorted by that sex's own count, descending, giving each
name a `male` row and a `female` row with a rank 1–1000 and a count that are
specific to that sex. `tools/names`' generator then takes every row with
`rank <= -given-per-sex` (default 500) for each sex and unions the two sets
— the "top-500 male and top-500 female given names" `tools/names/README.md`'s
own generator flag names.

**What that cut actually contains.** The derived rank is *within the same
1,000-name overall list*, not a per-sex ranking the Bureau ever published —
so "top-500 male" means the top 500 of those 1,000 names by male count, not
the 500 most common male names in the country. Because only 455 of the
1,000 names have more male than female bearers, male ranks 456–500 are
names that are mostly female, and the bottom of the male-ranked 500 is
thin: male ranks 495–500 are
`JOAN`, `SHELBY`, `CARMEN`, `BILLIE`, `ERIN` and `KENNEDY`, with male counts
from 9,797 down to 6,742, each of those a name far more commonly female —
`BOBBIE`, for comparison, is female rank 497 with a count of 48,842. Their
male counts are still real Census counts, not noise, so they belong in a
personal-data masking corpus regardless of which sex they read as more
often; the point is only that "top-500 male given names" is this
within-the-1,000 cut, not an independent per-sex top-500 the Bureau
ranked. `mask/words_corpus_test.go`'s pinned **958 given names** is the
union under exactly this definition.

`Names2020_LastNames_RaceHispanic_Top1000.xlsx` needs no such derivation:
its `RANK` and `FREQUENCY (COUNT)` columns are used as found.

## The CSVs

- `census2020_first_names_sex_top1000.csv` — `name,sex,rank,count`, 2,000
  rows (1,000 names × 2 sexes, per the derivation above).
- `census2020_last_names_top1000.csv` — `name,rank,count`, 1,000 rows.

Both are read-only input to `tools/names`' generator; nothing else in this
repository reads them directly.

## Extraction command

One-time, from the two `.xlsx` files above, in a scratch virtualenv (any
tool that reads `.xlsx` and writes CSV would do — this is documented, not a
maintained part of the build):

```sh
python3 -m venv venv
source venv/bin/activate
pip install openpyxl
python3 extract.py Names2020_FirstNames_Sex_Top1000.xlsx Names2020_LastNames_RaceHispanic_Top1000.xlsx
```

where `extract.py` is:

```python
#!/usr/bin/env python3
"""One-time extraction of the 2020 Census top-1000 name files to CSV."""
import csv
import sys
import openpyxl


def extract_first_names(path, out_path):
    wb = openpyxl.load_workbook(path, read_only=True, data_only=True)
    ws = wb[wb.sheetnames[0]]
    rows = list(ws.iter_rows(values_only=True))
    header = rows[2]
    assert header[0] == "FIRST NAME" and header[5] == "MALE" and header[6] == "FEMALE", header
    data = rows[3:1003]
    assert len(data) == 1000, len(data)

    # The workbook carries one overall RANK (by total FREQUENCY (COUNT)) and
    # two per-sex count columns, MALE and FEMALE, but no per-sex rank. Derive
    # one: sort the same 1,000 names by each sex's own count, descending,
    # ties (none observed) broken by name so the order is deterministic.
    by_male = sorted(data, key=lambda r: (-r[5], r[0]))
    by_female = sorted(data, key=lambda r: (-r[6], r[0]))

    out_rows = []
    for rank, r in enumerate(by_male, start=1):
        out_rows.append((r[0], "male", rank, r[5]))
    for rank, r in enumerate(by_female, start=1):
        out_rows.append((r[0], "female", rank, r[6]))
    out_rows.sort(key=lambda r: (r[1], r[2]))

    with open(out_path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["name", "sex", "rank", "count"])
        w.writerows(out_rows)


def extract_last_names(path, out_path):
    wb = openpyxl.load_workbook(path, read_only=True, data_only=True)
    ws = wb[wb.sheetnames[0]]
    rows = list(ws.iter_rows(values_only=True))
    header = rows[2]
    assert header[0] == "LAST NAME" and header[1] == "RANK" and header[2] == "FREQUENCY (COUNT)", header
    data = rows[3:1003]
    assert len(data) == 1000, len(data)

    out_rows = [(r[0], r[1], r[2]) for r in data]
    out_rows.sort(key=lambda r: r[1])

    with open(out_path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["name", "rank", "count"])
        w.writerows(out_rows)


if __name__ == "__main__":
    first_xlsx, last_xlsx = sys.argv[1], sys.argv[2]
    extract_first_names(first_xlsx, "census2020_first_names_sex_top1000.csv")
    extract_last_names(last_xlsx, "census2020_last_names_top1000.csv")
```

Both source `.xlsx` files put their header row at (0-indexed) row 2 and
their 1,000 data rows at rows 3–1002 — a title row and a blank row precede
the header in each workbook, checked by the `assert`s above.

## The generator

`go run ./tools/names` (flags `-given-per-sex 500 -surnames 1000` by
default): reads the two CSVs above, lowercases every name, drops (and
counts) any token that is not `^[a-z]+$` after lowercasing — the fetch
above found none in either file, and `tools/names/main_test.go`'s
`TestNoDrops` asserts that stays true — asserts every kept word is at most
32 bytes (`mask.maxFit`, `mask/words.go`) and that neither list has an
internal duplicate, and writes `mask/words_corpus.go`, alphabetically within
each list. `go run ./tools/names -check` regenerates into memory instead of
writing, and exits 1 if the committed file differs. `mask/words_corpus_test.go`
pins the resulting counts (958 given names, 1,000 surnames), the character
class and the length bound, so a silent regeneration with a different cut
fails loudly there too.
