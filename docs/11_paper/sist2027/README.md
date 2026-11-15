# Conference version for IEEE SIST 2027

Short version of the article in `docs/11_paper/manuscript.md`, prepared for the 2027 IEEE 7th International Conference on Smart Information Systems and Technologies (SIST), Astana, 12–14 May 2027, which the supervisor recommended on 24 Oct 2026.

- Call for papers: https://sist.astanait.edu.kz/?page_id=42
- Submission guidelines: https://sist.astanait.edu.kz/?page_id=368
- Submission system: Microsoft CMT, https://cmt3.research.microsoft.com/IEEESIST2027
- Paper submission deadline: 12 February 2027; notification of acceptance: 30 April 2027.

The requirements below were read from the conference pages and the template file on 15 Nov 2026. The pages still mix 2026 and 2027 labels (for example the camera-ready dates), so check them again before submitting.

## Relevance

The paper fits Track 4, Computational Intelligence: "Applications of Computational Intelligence in Medicine, Education, Agriculture and Smart Cities" and "Artificial Intelligence". A possible secondary subject area is Track 1, Computer Science: "Specification, design, prototyping, and testing methods and tools". The call asks for new advances, theories or applications with an analysis of results; the paper offers an evaluation method (paired simulated worlds), an open simulator, and measured results for an anomaly-detection application in education.

## Requirements and how the paper meets them

| Requirement | Source | State |
|---|---|---|
| Standard IEEE two-column conference format, A4, no page numbers | template, guidelines | met: `IEEEtran`, `conference,a4paper` |
| 4–6 pages | call for papers | met: 6 pages |
| English only | call for papers | met; American spelling as in the template |
| PDF, at most 10 MB, uploaded to CMT | CMT guidelines | met: `paper.pdf` |
| Structure: title, abstract, keywords, affiliations; introduction, related work, methods, results (and future work), conclusion; acknowledgment optional; references | guidelines | met; Section V is "Discussion and Future Work" |
| Title in title case, no symbols or math in title and abstract | template | met |
| At least 10 references | call for papers | met: 32 |
| At least 30 % of references from Scopus or Web of Science | call for papers | expected to be met: 24 of the 32 references are journal articles (Springer, Wiley, SAGE, IEEE, ACM, Elsevier, Frontiers, MDPI, PLOS, MIT Press) and 10 are needed; confirm the indexing of the final list in Scopus before submission |
| Self-citations at most 20 % | call for papers | met: none |
| At most 6 authors; one author may submit up to 3 papers | call for papers | met: two authors |
| References numbered in order of citation; "et al." for six or more authors | template | met: `IEEEtran.bst` with the control entry in `refs.bib` |
| Figure labels in words, captions below figures, table heads above tables | template | met |
| Original, unpublished, not under review elsewhere; screened with CrossCheck | guidelines | see the note on the public repository below |
| At least one author registers and presents (15 min + 5 min questions) | guidelines | to arrange |

## What the authors still have to do

1. **Author block.** The paper has two authors, the student (first author) and the supervisor, whose name is written as in his earlier publications. E-mail addresses are required by the template but are not kept in this public repository: copy `authors.example.tex` to `authors.local.tex` (git-ignored), fill it in, and build `submission.tex`; the result, `submission.pdf`, is the file to upload. ORCID identifiers are optional.
2. **Acknowledgment.** The section thanks the team member who developed the detector pipeline and the original generator. Still to add: funding, if any, and the disclosure that IEEE requires when AI-generated content is used in a paper (the IEEE policy asks for it in the acknowledgments section). The place is marked in `paper.tex`.
3. **Similarity check.** The long version of the article is in this public repository. It has not been published, but a similarity service may index it. If the conference asks, state that the repository is the authors' own supplementary material; alternatively move the long draft out of the public tree before submission.
4. **Repository state.** The paper says that the simulator, the calibration and all results are available at the repository URL, and that the list of surveyed studies is published with the code. This holds only after the branch `LMS-35-behaviour-simulation` is merged into `main`, so merge it before submitting. The repository has no licence file; add one if the simulator is to be called open.
5. **Subject areas in CMT.** Primary: Computational Intelligence (applications in education); secondary: Computer Science (testing methods and tools).
6. **Registration fee.** According to the supervisor's message of 24 Oct 2026, the fee is covered by the supervisor when he is a co-author.
7. **Later journal version.** The long manuscript can be submitted to a journal afterwards as an extended version; IEEE expects such a version to cite the conference paper and to contain substantial new material, which it does (sensitivity analysis, protocol variants, real-data strata, scoping survey).

## Build

Run from the repository root:

```bash
.venv/bin/python docs/11_paper/sist2027/make_refs.py
```

```bash
.venv/bin/python docs/11_paper/sist2027/make_fig_dose.py
```

```bash
tectonic -X compile docs/11_paper/sist2027/paper.tex
```

For the upload version with the e-mail addresses:

```bash
tectonic -X compile docs/11_paper/sist2027/submission.tex
```

`make_refs.py` copies the cited entries from `../references.bib` into `refs.bib` (without the `note` fields, which IEEEtran would print). `make_fig_dose.py` redraws the dose figure from `ml/experiments/inflation/results/stages/fi_dose.json`. The source also compiles with pdfLaTeX, XeLaTeX or LuaLaTeX (for example on Overleaf, after uploading `paper.tex`, `refs.bib` and `fig_dose.pdf`).

Every number in the paper was checked on 15 Nov 2026 against the generated tables in `ml/experiments/inflation/results/tables/` and the stage outputs in `results/stages/` (about 290 values); the 29 references that have a DOI were compared with their Crossref records. The acknowledgment can take about 45 more words (funding, AI-use disclosure) before the paper exceeds six pages.
