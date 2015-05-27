package drawing

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/pbnjay/lollipops/data"
)

const svgHeader = `<?xml version='1.0'?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%f" height="%f">
<defs>
  <filter id="ds" x="0" y="0">
    <feOffset in="SourceAlpha" dx="2" dy="2" />
    <feComponentTransfer><feFuncA type="linear" slope="0.2"/></feComponentTransfer>
    <feGaussianBlur result="blurOut" stdDeviation="1" />
    <feBlend in="SourceGraphic" in2="blurOut" mode="normal" />
  </filter>
  <pattern id="disordered-hatch" patternUnits="userSpaceOnUse" width="4" height="4">
    <path d="M-1,1 l2,-2 M0,4 l4,-4 M3,5 l2,-2" stroke="#000000" opacity="0.3" />
  </pattern>
</defs>
`
const svgFooter = `</svg>`

func DrawSVG(w io.Writer, changelist []string, g *data.PfamGraphicResponse) {
	DefaultSettings.DrawSVG(w, changelist, g)
}

// DrawSVG writes the SVG XML document to w, with the provided changes in changelist
// and Pfam domain/region information in g. If GraphicWidth=0, the AutoWidth is called
// to determine the best diagram width to fit all labels.
func (s *Settings) DrawSVG(w io.Writer, changelist []string, g *data.PfamGraphicResponse) {
	if s.GraphicWidth == 0 {
		s.GraphicWidth = s.AutoWidth(g)
	}
	aaLen, _ := g.Length.Int64()
	scale := (s.GraphicWidth - s.Padding*2) / float64(aaLen)
	popSpace := int((s.LollipopRadius + 2) / scale)
	aaSpace := int(20 / scale)
	startY := s.Padding
	if s.ShowLabels {
		startY += s.Padding // add some room for labels
	}

	pops := TickSlice{}
	col := s.SynonymousColor
	ht := s.DomainHeight + s.Padding*2
	if len(changelist) > 0 {
		popMatch := make(map[string]int)
		// parse changelist and check if lollipops need staggered
		for i, chg := range changelist {
			cnt := 1
			cpos := stripChangePos.FindStringSubmatch(chg)
			spos := 0
			col = s.SynonymousColor
			if cpos[3] != "" && cpos[3] != "=" && cpos[3] != cpos[1] {
				col = s.MutationColor
			}
			if strings.Contains(chg, "@") {
				parts := strings.SplitN(chg, "@", 2)
				fmt.Sscanf(parts[1], "%d", &cnt)
				chg = parts[0]
			}
			if strings.Contains(chg, "#") {
				parts := strings.SplitN(chg, "#", 2)
				col = "#" + parts[1]
				chg = parts[0]
			}
			changelist[i] = chg
			fmt.Sscanf(cpos[2], "%d", &spos)
			col = strings.ToLower(col)
			if idx, f := popMatch[chg+col]; f {
				pops[idx].Cnt += cnt
			} else {
				popMatch[chg+col] = len(pops)
				pops = append(pops, Tick{spos, -i, cnt, col})
			}
		}
		sort.Sort(pops)
		maxStaggered := s.LollipopRadius + s.LollipopHeight
		for pi, pop := range pops {
			h := s.LollipopRadius + s.LollipopHeight
			for pj := pi + 1; pj < len(pops); pj++ {
				if pops[pj].Pos-pop.Pos > popSpace {
					break
				}
				h += 0.5 + (pop.Radius(s) * 3.0)
			}
			if h > maxStaggered {
				maxStaggered = h
			}
		}
		ht += maxStaggered
		startY += maxStaggered - (s.LollipopRadius + s.LollipopHeight)
	}
	if !s.HideAxis {
		ht += s.AxisPadding + s.AxisHeight
	}

	ticks := []Tick{
		Tick{Pos: 0, Pri: 0},           // start isn't very important (0 is implied)
		Tick{Pos: int(aaLen), Pri: 99}, // always draw the length in the axis
	}

	fmt.Fprintf(w, svgHeader, s.GraphicWidth, ht)

	if len(pops) > 0 {
		poptop := startY + s.LollipopRadius
		popbot := poptop + s.LollipopHeight
		startY = popbot - (s.DomainHeight-s.BackboneHeight)/2

		// draw lollipops
		for pi, pop := range pops {
			ticks = append(ticks, Tick{Pos: pop.Pos, Pri: 10})
			spos := s.Padding + (float64(pop.Pos) * scale)

			mytop := poptop
			for pj := pi + 1; pj < len(pops); pj++ {
				if pops[pj].Pos-pop.Pos > popSpace {
					break
				}
				mytop -= 0.5 + (pops[pj].Radius(s) * 3.0)
			}
			fmt.Fprintf(w, `<line x1="%f" x2="%f" y1="%f" y2="%f" stroke="#BABDB6" stroke-width="2"/>`, spos, spos, mytop, popbot)
			fmt.Fprintf(w, `<a xlink:title="%s"><circle cx="%f" cy="%f" r="%f" fill="%s" /></a>`,
				changelist[-pop.Pri], spos, mytop, pop.Radius(s), pop.Col)

			if s.ShowLabels {
				fmt.Fprintf(w, `<g transform="translate(%f,%f) rotate(-30)">`,
					spos, mytop)
				chg := changelist[-pop.Pri]
				if pop.Cnt > 1 {
					chg = fmt.Sprintf("%s (%d)", chg, pop.Cnt)
				}
				fmt.Fprintf(w, `<text style="font-size:10px;font-family:sans-serif;fill:#555;" text-anchor="middle" x="0" y="%f">%s</text></g>`,
					(pop.Radius(s) * -1.5), chg)
			}
		}
	}

	// draw the backbone
	fmt.Fprintf(w, `<a xlink:title="%s, %s (%daa)"><rect fill="#BABDB6" x="%f" y="%f" width="%f" height="%f"/></a>`,
		g.Metadata.Identifier, g.Metadata.Description, aaLen,
		s.Padding, startY+(s.DomainHeight-s.BackboneHeight)/2, s.GraphicWidth-(s.Padding*2), s.BackboneHeight)

	disFill := "url(#disordered-hatch)"
	if s.SolidFillOnly {
		disFill = `#000;" opacity="0.15`
	}
	if !s.HideMotifs {
		// draw transmembrane, signal peptide, coiled-coil, etc motifs
		for _, r := range g.Motifs {
			if r.Type == "pfamb" {
				continue
			}
			if r.Type == "disorder" && s.HideDisordered {
				continue
			}
			sstart, _ := r.Start.Float64()
			swidth, _ := r.End.Float64()

			sstart *= scale
			swidth = (swidth * scale) - sstart

			fmt.Fprintf(w, `<a xlink:title="%s">`, r.Type)
			if r.Type == "disorder" {
				// draw disordered regions with a understated diagonal hatch pattern
				fmt.Fprintf(w, `<rect fill="%s" x="%f" y="%f" width="%f" height="%f"/>`, disFill,
					s.Padding+sstart, startY+(s.DomainHeight-s.BackboneHeight)/2, swidth, s.BackboneHeight)
			} else {
				fmt.Fprintf(w, `<rect fill="%s" x="%f" y="%f" width="%f" height="%f" filter="url(#ds)"/>`, BlendColorStrings(r.Color, "#FFFFFF"),
					s.Padding+sstart, startY+(s.DomainHeight-s.MotifHeight)/2, swidth, s.MotifHeight)

				tstart, _ := r.Start.Int64()
				tend, _ := r.End.Int64()
				ticks = append(ticks, Tick{Pos: int(tstart), Pri: 1})
				ticks = append(ticks, Tick{Pos: int(tend), Pri: 1})
			}
			fmt.Fprintln(w, `</a>`)
		}
	}

	// draw the curated domains
	for _, r := range g.Regions {
		sstart, _ := r.Start.Float64()
		swidth, _ := r.End.Float64()

		ticks = append(ticks, Tick{Pos: int(sstart), Pri: 5})
		ticks = append(ticks, Tick{Pos: int(swidth), Pri: 5})

		sstart *= scale
		swidth = (swidth * scale) - sstart

		fmt.Fprintf(w, `<g transform="translate(%f,%f)"><a xlink:href="%s" xlink:title="%s">`, s.Padding+sstart, startY, "http://pfam.xfam.org"+r.Link, r.Metadata.Description)
		fmt.Fprintf(w, `<rect fill="%s" x="0" y="0" width="%f" height="%f" filter="url(#ds)"/>`, r.Color, swidth, s.DomainHeight)
		if swidth > 10 {
			if len(r.Metadata.Description) > 1 && float64(MeasureFont(r.Metadata.Description, 12)) < (swidth-s.TextPadding) {
				// we can fit the full description! nice!
				fmt.Fprintf(w, `<text style="font-size:12px;font-family:sans-serif;fill:#ffffff;" text-anchor="middle" x="%f" y="%f">%s</text>`, swidth/2.0, 4+s.DomainHeight/2, r.Metadata.Description)
			} else if float64(MeasureFont(r.Text, 12)) < (swidth - s.TextPadding) {
				fmt.Fprintf(w, `<text style="font-size:12px;font-family:sans-serif;fill:#ffffff;" text-anchor="middle" x="%f" y="%f">%s</text>`, swidth/2.0, 4+s.DomainHeight/2, r.Text)
			} else {
				didOutput := false
				if strings.IndexFunc(r.Text, unicode.IsPunct) != -1 {

					// if the label is too long, we assume the most
					// informative word is the last one, but if that
					// still won't fit we'll move up
					//
					// Example: TP53 has P53_TAD and P53_tetramer
					// domains but boxes aren't quite large enough.
					// Showing "P53..." isn't very helpful.

					parts := strings.FieldsFunc(r.Text, unicode.IsPunct)
					pre := ".."
					post := ""
					for i := len(parts) - 1; i >= 0; i-- {
						if i == 0 {
							pre = ""
						}
						if float64(MeasureFont(pre+parts[i]+post, 12)) < (swidth - s.TextPadding) {
							fmt.Fprintf(w, `<text style="font-size:12px;font-family:sans-serif;fill:#ffffff;" text-anchor="middle" x="%f" y="%f">%s</text>`, swidth/2.0, 4+s.DomainHeight/2, pre+parts[i]+post)
							didOutput = true
							break
						}
						post = ".."
					}
				}

				if !didOutput && swidth > 40 {
					sub := r.Text
					for mx := len(r.Text) - 2; mx > 0; mx-- {
						sub = strings.TrimFunc(r.Text[:mx], unicode.IsPunct) + ".."
						if float64(MeasureFont(sub, 12)) < (swidth - s.TextPadding) {
							break
						}
					}

					fmt.Fprintf(w, `<text style="font-size:12px;font-family:sans-serif;fill:#ffffff;" text-anchor="middle" x="%f" y="%f">%s</text>`, swidth/2.0, 4+s.DomainHeight/2, sub)
				}
			}
		}
		fmt.Fprintln(w, `</a></g>`)
	}

	if !s.HideAxis {
		startY += s.DomainHeight + s.AxisPadding
		fmt.Fprintln(w, `<g class="axis">`)
		fmt.Fprintf(w, `<line x1="%f" x2="%f" y1="%f" y2="%f" stroke="#AAAAAA" />`, s.Padding, s.GraphicWidth-s.Padding, startY, startY)
		fmt.Fprintf(w, `<line x1="%f" x2="%f" y1="%f" y2="%f" stroke="#AAAAAA" />`, s.Padding, s.Padding, startY, startY+(s.AxisHeight/3))

		ts := TickSlice(ticks)
		sort.Sort(ts)
		lastDrawn := 0
		for i, t := range ts {
			if lastDrawn > 0 && (t.Pos-lastDrawn) < aaSpace {
				continue
			}
			j := ts.NextBetter(i, aaSpace)
			if i != j {
				continue
			}
			lastDrawn = t.Pos
			x := s.Padding + (float64(t.Pos) * scale)
			fmt.Fprintf(w, `<line x1="%f" x2="%f" y1="%f" y2="%f" stroke="#AAAAAA" />`, x, x, startY, startY+(s.AxisHeight/3))
			fmt.Fprintf(w, `<text style="font-size:10px;font-family:sans-serif;fill:#000000;" text-anchor="middle" x="%f" y="%f">%d</text>`, x, startY+s.AxisHeight, t.Pos)
		}

		fmt.Fprintln(w, "</g>")
	}

	fmt.Fprintln(w, svgFooter)
}
