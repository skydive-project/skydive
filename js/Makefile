.PHONY: all
all: ../statics/js/bundle.js browser.js api.js

npm.install:

api.js: api.ts
	npm install
	PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 api.ts

browser.js: browser.ts api.ts
	npm install
	PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 browser.ts

../statics/js/bundle.js: browser.js
	PATH=`npm bin`:$$PATH browserify browser.js -o ../statics/js/bundle.js

.PHONY: clean
clean:
	rm -f ../statics/js/bundle.js browser.js api.js
