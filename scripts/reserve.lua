local seat_key = KEYS[1]
local buyers_key = KEYS[2]
local passenger_id = ARGV[1]
local pnr_number = ARGV[2]

-- 1. Check if passenger already booked
if redis.call("SISMEMBER", buyers_key, passenger_id) == 1 then
    return -1 -- Duplicate
end

-- 2. Check remaining Tatkal seats
local available_seats = tonumber(redis.call("GET", seat_key))

if not available_seats or available_seats <= 0 then
    return 0 -- REGRET / Sold Out
end

-- 3. Atomic Allocation (Indivisible)
redis.call("DECR", seat_key)
redis.call("SADD", buyers_key, passenger_id)

-- 4. Hold Seat for 10 minutes (600s) to pay
redis.call("SETEX", "pnr:" .. pnr_number, 600, passenger_id)

return 1 -- CNF (Confirmed!)