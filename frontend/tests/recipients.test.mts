import { layoutRecipients } from '../src/composables/recipients.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  PASS  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

const layout = (widths: number[], available: number, chip = 60, sep = 6) =>
  layoutRecipients(widths, available, chip, sep)

// Everything fits: no chip, every address visible.
{
  const got = layout([40, 40, 40], 400)
  check('all fit → nothing hidden', got.visible === 3 && got.hidden === 0)
}

// The third address would push the chip onto the next line, so it is dropped and
// the chip stands for it: two addresses plus the chip need 40 + 6 + 40 + 6 + 60 =
// 152, and only 140 is available.
{
  const got = layout([40, 40, 40], 140)
  check('chip reservation ends the run', got.visible === 1 && got.hidden === 2)
}

// 152 is exactly enough for two addresses and the chip; one pixel less is not.
{
  const widths = [40, 40, 40, 40]
  const exact = layout(widths, 152)
  check('exact boundary still keeps two', exact.visible === 2 && exact.hidden === 2)
  const tight = layout(widths, 151)
  check('one pixel short drops an address', tight.visible === 1 && tight.hidden === 3)
}

// The last address never has to leave room for the chip — if it fits, there is no
// chip at all.
{
  const got = layout([100, 100], 206)
  check('last address needs no chip room', got.visible === 2 && got.hidden === 0)
}

// A single address wider than the row: keep it and clip, never an empty row.
{
  const got = layout([500], 200)
  check('one oversized address is still shown', got.visible === 1 && got.hidden === 0)
  const many = layout([500, 30, 30], 200)
  check('oversized first address keeps a chip for the rest', many.visible === 1 && many.hidden === 2)
}

// Degenerate inputs must not produce a negative or oversized count.
{
  check('empty list', layout([], 400).visible === 0 && layout([], 400).hidden === 0)
  check('unmeasured width shows everything', layout([40, 40], 0).visible === 2)
  check('negative width shows everything', layout([40, 40], -10).visible === 2)
}

// A realistic company mail: 30 recipients, 800px pane minus a 40px label.
{
  const widths = Array.from({ length: 30 }, (_, i) => 120 + (i % 5) * 10)
  const got = layout(widths, 760, 90, 6)
  check('big list collapses to one line', got.visible > 0 && got.visible < 10)
  check('nothing is lost', got.visible + got.hidden === 30)
}

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
