package ai

import (
	"sort"

	"powergame/engine"
)

// Early in the game cities, not plants, hold a network back: a plant that
// costs most of our cash leaves nothing to fuel it or to build the cities
// it could power, and income stalls. So before buying a plant (or deciding
// how high to bid) the bot plays the next few rounds forward on a simple
// budget, with and without the plant, and only pays what the plant adds.

// outlookRounds is how many rounds the budget looks ahead.
const outlookRounds = 3

// cityAssetValue is how much a city still owned at the end of the outlook
// is worth relative to what it cost: cities keep paying every round.
const cityAssetValue = 0.9

// Cash beyond comfortCash is worth less and less: a pile of money only
// breaks ties at the end, while plants and cities win the game.
const (
	comfortCash = 60.0
	spareCash   = 0.4
)

// endPowerValue is what each city we could power in the final round is
// worth once the end of the game is in sight.
const endPowerValue = 25.0

// cityCost estimates what each of our next few cities will cost. It is
// worked out once per decision.
func (b *bot) cityCost() float64 {
	if b.cc > 0 {
		return b.cc
	}
	// Average of the three cheapest builds, without sorting them all.
	low := [3]int{-1, -1, -1}
	for _, c := range b.g.BuildCosts(b.p) {
		if c < 0 {
			continue
		}
		for i := range low {
			if low[i] < 0 || c < low[i] {
				copy(low[i+1:], low[i:2])
				low[i] = c
				break
			}
		}
	}
	b.cc = 30
	if low[0] >= 0 {
		sum, n := 0, 0
		for _, c := range low {
			if c >= 0 {
				sum += c
				n++
			}
		}
		b.cc = max(10, float64(sum)/float64(n))
	}
	return b.cc
}

// fuelPlan lists a set's plants, cheapest fuel per city first.
type fuelPlan []struct{ cities, cost float64 }

func (b *bot) fuelPlan(set []int) fuelPlan {
	var p fuelPlan
	for _, n := range set {
		if pl, ok := engine.PlantByNum(n); ok {
			p = append(p, struct{ cities, cost float64 }{float64(pl.Cities), float64(pl.Input) * b.price(pl.Kind)})
		}
	}
	sort.Slice(p, func(i, j int) bool { return p[i].cost*p[j].cities < p[j].cost*p[i].cities })
	return p
}

// cost is what it costs at today's prices to power `cities` cities.
func (p fuelPlan) cost(cities int) float64 {
	need, total := float64(cities), 0.0
	for _, r := range p {
		if need <= 0 {
			break
		}
		total += r.cost
		need -= r.cities
	}
	return total
}

// outlook plays the next rounds forward with plants `set` and `money`:
// each round it builds as many cities as it can fuel and afford (up to
// what the plants can power) and collects income. It returns the money
// left plus the lasting value of the network.
func (b *bot) outlook(set []int, money float64) float64 {
	return b.outlookPlan(b.fuelPlan(set), capacity(set), money)
}

func (b *bot) outlookPlan(plan fuelPlan, capa int, money float64) float64 {
	cities := len(b.me.Cities)
	cc := b.cityCost()
	for r := 0; r < outlookRounds; r++ {
		k := 0
		for cities+k < capa && plan.cost(cities+k+1)+float64(k+1)*cc <= money {
			k++
		}
		powered := min(capa, cities+k)
		for powered > 0 && plan.cost(powered)+float64(k)*cc > money {
			powered-- // cannot fuel everything: run what we can afford
		}
		money -= plan.cost(powered) + float64(k)*cc
		cities += k
		money += float64(engine.Income(powered))
	}
	value := money
	if money > comfortCash {
		value = comfortCash + (money-comfortCash)*spareCash
	}
	value += float64(cities) * cc * cityAssetValue
	if b.endInSight() {
		// The winner is whoever powers the most cities in the last round.
		value += endPowerValue * float64(min(capa, max(cities, b.g.EndTrigger())))
	}
	return value
}

// endInSight reports whether the game could end within a few rounds.
func (b *bot) endInSight() bool {
	return b.g.Step >= 3 || b.g.MaxCities() >= b.g.EndTrigger()-6
}

// plantGain is how much better off the bot ends up if it buys plant num
// for price, compared with not buying it.
func (b *bot) plantGain(num, price int) float64 {
	return b.gainWith(b.bestSet(num))(price)
}

// gainWith returns plantGain for a fixed new plant set as a function of
// price, reusing the sorted fuel plan across prices.
func (b *bot) gainWith(set []int) func(price int) float64 {
	if !b.haveBase {
		b.base, b.haveBase = b.outlook(b.me.Plants, float64(b.me.Money)), true
	}
	plan, capa := b.fuelPlan(set), capacity(set)
	return func(price int) float64 {
		return b.outlookPlan(plan, capa, float64(b.me.Money-price)) - b.base
	}
}

// cashMaxBid is the most the bot should pay for plant num: the highest
// price at which it still beats both buying nothing and buying the best
// other plant on sale at its minimum price. Without that second check a
// bot that needs capacity would pay almost anything for the first plant
// it bids on, even with a similar plant going cheap next to it.
func (b *bot) cashMaxBid(num int) int {
	gain := b.gainWith(b.bestSet(num))
	alt := 0.0
	for _, n := range b.g.ActualMarket() {
		if n != num && n <= b.me.Money {
			alt = max(alt, b.plantGain(n, n))
		}
	}
	if num > b.me.Money || gain(num) <= alt {
		return num - 1
	}
	lo, hi := num, b.me.Money // gain(lo) > alt
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if gain(mid) > alt {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// fuelGoal is how many cities to buy fuel for: the ones we have plus the
// ones we can still afford to build and fuel this round.
func (b *bot) fuelGoal() int {
	cities := len(b.me.Cities)
	capa := capacity(b.me.Plants)
	cc := b.cityCost()
	money := float64(b.me.Money)
	k := 0
	plan := b.fuelPlan(b.me.Plants)
	for cities+k < capa && plan.cost(cities+k+1)+float64(k+1)*cc <= money {
		k++
	}
	return cities + k
}
