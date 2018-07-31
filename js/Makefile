.PHONY: all
all: ../statics/js/bundle.js browser.js api.js

api.js: api.ts
	tsc --module commonjs --target ES5 api.ts

browser.js: browser.ts api.ts
	tsc --module commonjs --target ES5 browser.ts

../statics/js/bundle.js: browser.js
	browserify browser.js -o ../statics/js/bundle.js

.PHONY: clean
clean:
	rm -f ../statics/js/bundle.js browser.js api.js
