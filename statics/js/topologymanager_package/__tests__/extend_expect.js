const expect = require('expect');

function removeKeysFromHashMap(hashmap, keys = []) {
    return Object.keys(hashmap).reduce((accum, currentKey) => {
        if (keys.includes(currentKey)) {
            return accum;
        }
        if (typeof hashmap[currentKey] === "object") {
            accum[currentKey] = removeKeysFromHashMap(hashmap[currentKey], keys);
        } else {
            accum[currentKey] = hashmap[currentKey];
        }
        return accum;
    }, {});
}

expect.extend({
    toEqualHashMap(actual, expected, skipKeys = [], message) {
        // remove keys from hashes
        actual = removeKeysFromHashMap(actual, skipKeys);
        expected = removeKeysFromHashMap(expected, skipKeys);
        // compare these two hashes
        try {
            expect(actual).toEqual(expected);
        } catch (error) {
            throw error;
        }
        return { pass: true };
    }
});
