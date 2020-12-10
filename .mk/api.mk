.typescript: statics/js/bundle.js js/browser.js js/api.js

graffiti/js/api.js: graffiti/js/api.ts
	cd graffiti/js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 api.ts

js/api.js: js/api.ts
	cd js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 api.ts

js/browser.js: js/browser.ts js/api.ts
	cd js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 browser.ts

statics/js/bundle.js: js/browser.js
	cd js && PATH=`npm bin`:$$PATH browserify browser.js -o ../statics/js/bundle.js

.PHONY: .typescript.clean
.typescript.clean:
	rm -f statics/js/bundle.js js/browser.js js/api.js
