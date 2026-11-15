local seat_key = KEYS[1]
local buyers_key = KEYS[2]
local passenger_id = ARGV[1]
local pnr_number = ARGV[2]

-- 1. Check if passenger already booked (duplicate check happens first)
if redis.call("SISMEMBER", buyers_key, passenger_id) == 1 then
    return -1 -- ALREADY_BOOKED
end

-- 2. Check remaining Tatkal seats
local available_seats = tonumber(redis.call("GET", seat_key))

if not available_seats or available_seats <= 0 then
    return 0 -- REGRET / Sold Out
end

-- 3. Atomic allocation: decrement seat and record passenger together
redis.call("DECR", seat_key)
redis.call("SADD", buyers_key, passenger_id)

-- 4. Store PNR record (auto-expires in 600s). NOTE: this does NOT release the seat.
redis.call("SETEX", "pnr:" .. pnr_number, 600, passenger_id)

return 1 -- CONFIRMED
