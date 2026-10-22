# Power Game rules

The same rules are in the app (**How to play**, or **Menu → How to play**).
The numbers here match the engine in `engine/game.go`.

## Goal
Buy power plants, keep them fuelled and connect cities to your network. When
the game ends, the player who powers the most cities in the final round wins.

## Setup and player count
Everyone starts with $50, no plants and no cities. Only some regions of the
map are in play. They are picked at random or chosen on the setup screen, and
they always touch each other.

| Players | Regions | Max plants | Step 2 at | Game ends at | Plants removed |
|---|---|---|---|---|---|
| 2 | 3 | 4 | 10 cities | 21 cities | 8 |
| 3 | 3 | 3 | 7 | 17 | 8 |
| 4 | 4 | 3 | 7 | 17 | 4 |
| 5 | 5 | 3 | 7 | 15 | 0 |
| 6 | 5 | 3 | 6 | 14 | 0 |

"Plants removed" are taken out of the deck at random at the start, and nobody
sees which ones. The plant market starts with plants 3–10. Plant 13 is on top
of the deck and the Step 3 card at the bottom. The fuel market starts with
coal from $1, oil from $3, garbage from $7 and uranium at $14 and $16.

## A round
1. Turn order
2. Plant auction
3. Buy fuel
4. Build
5. Power and get paid

### 1. Turn order
- Most cities goes first. Ties go to whoever owns the biggest-numbered plant.
- Round 1 starts in random order, then turn order is set again by plant size
  straight after the round-1 auction.
- Going first cuts both ways: you choose plants first, but you buy fuel and
  build last.

### 2. Plant auction
- The top row of the market is for sale; the bottom row is what comes next.
  In step 3 all six plants are for sale.
- In turn order, each player who hasn't bought a plant this round either picks
  a plant and opens bidding at its number or more, or passes. Passing means no
  plant this round. In round 1 everyone must buy one, unless they can't afford any.
- Bidding goes round in turn order. Each bidder raises or drops out of this
  auction. The last one in pays their bid and takes the plant.
- Each player buys at most one plant per round. If the opener loses, they
  choose again.
- You can own the "Max plants" from the table. Buying one more means scrapping
  an older plant. Its fuel moves to your other plants if there's room, and the
  rest goes back to the bank.
- After each sale a new plant is drawn and the market is re-sorted.
- If nobody buys a plant all round, the cheapest plant leaves the game.
- Any market plant numbered no higher than the biggest network (in cities)
  leaves the game and is replaced.

### Reading a plant card
- The big number is the plant's number and minimum bid.
- "2🪨 → 3🏠" burns 2 coal to power up to 3 cities.
- 🪨🛢️ hybrids burn coal, oil or any mix. 🌱 eco plants need no fuel.
- A plant stores up to twice the fuel it burns.

### 3. Buy fuel
- Players buy in reverse turn order.
- Coal, oil and garbage are sold three units per price step from $1 to $8.
  Uranium is one unit per step: $1–$8, then $10, $12, $14 and $16. You always
  pay for the cheapest units left.
- Add fuel to an order, then **Confirm**. **Reset** clears it. The whole order
  is checked together (stock, storage, money) and either all of it is bought
  or none of it.
- You can only buy what your plants can store.

### 4. Build
- Players build in reverse turn order.
- Your first house can go in any city in play and costs $10.
- Each later house costs the house price plus the cheapest road from your
  network. Road costs are the numbers on the map.
- House prices: the 1st house in a city costs $10, the 2nd $15 and the 3rd $20.
- Cities take 1 house in step 1, 2 in step 2 and 3 in step 3. You can never
  have two houses in the same city.
- Roads can pass through any city in play.
- You can build as many houses as you can afford, one at a time.

### 5. Power and get paid
- In turn order, choose which plants to run, or none to save fuel. With a
  hybrid you also choose the coal/oil mix.
- A running plant burns its fuel and powers up to its number of cities. You
  can't power more cities than you own.

| Powered | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Income | $10 | $22 | $33 | $44 | $54 | $64 | $73 | $82 | $90 | $98 | $105 |

| Powered | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20+ |
|---|---|---|---|---|---|---|---|---|---|---|
| Income | $112 | $118 | $124 | $129 | $134 | $138 | $142 | $145 | $148 | $150 |

## End of round
- Fuel restocks from the bank. The bank only holds fuel that isn't in the
  market or on plants, so restock can come up short.
- Steps 1–2: the most expensive market plant goes under the deck and a new one
  is drawn. Step 3: the cheapest plant leaves the game and a new one is drawn.

Standard restock per round (coal / oil / garbage / uranium):

| Players | Step 1 | Step 2 | Step 3 |
|---|---|---|---|
| 2 | 3 / 2 / 1 / 1 | 4 / 2 / 2 / 1 | 3 / 4 / 3 / 1 |
| 3 | 4 / 2 / 1 / 1 | 5 / 3 / 2 / 1 | 3 / 4 / 3 / 1 |
| 4 | 5 / 3 / 2 / 1 | 6 / 4 / 3 / 2 | 4 / 5 / 4 / 2 |
| 5 | 5 / 4 / 3 / 2 | 7 / 5 / 3 / 3 | 5 / 6 / 5 / 2 |
| 6 | 7 / 5 / 3 / 2 | 9 / 6 / 5 / 3 | 6 / 7 / 6 / 3 |

On the setup screen, **Fuel restock** can switch these to Scarce (half, at least 1),
Plentiful (one and a half times) or your own numbers (0–24 each, uranium 0–12).

## Steps
- **Step 1:** one house per city.
- **Step 2** starts after the build phase in which someone reaches the
  "Step 2 at" count. The cheapest plant leaves the market, and cities now take
  two houses.
- **Step 3** starts when the Step 3 card is drawn. At the end of that phase the
  cheapest plant leaves, the deck is shuffled, and the market becomes six
  plants, all for sale. Cities now take three houses.

## End of the game
- The game ends after the build phase in which someone reaches the "Game ends
  at" count. Everyone still powers cities and gets paid that round.
- The winner powers the most cities in that final round. Ties go to the most
  cash, then the most cities.

## Maps
Classic, Lakeside and Ridge are fixed maps. Random generates a new map and
gives it a code you can share. Enter the code to replay that map. Every map
has six regions of at least seven cities.
