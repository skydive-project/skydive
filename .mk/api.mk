API_JS_FILES := \
	graffiti/js/api.js \
	js/api.js \
	js/browser.js \
	statics/ui/js/bundle.js

.typescript: $(API_JS_FILES)

graffiti/js/api.js: graffiti/js/api.ts
	cd graffiti/js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 api.ts

js/api.js: graffiti/js/api.ts js/api.ts
	cd js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 api.ts

js/browser.js: graffiti/js/api.js js/api.js js/browser.ts
	cd js && npm ci && PATH=`npm bin`:$$PATH tsc --module commonjs --target ES5 browser.ts

statics/ui/js/bundle.js: js/browser.js
	cd js && npm ci && PATH=`npm bin`:$$PATH browserify browser.js -o ../statics/ui/js/bundle.js

.PHONY: .typescript.touch
.typescript.touch:
	@echo $(API_JS_FILES) | xargs touch

.PHONY: .typescript.clean
.typescript.clean:
	rm -f graffiti/js/api.js js/api.js js/browser.js statics/ui/js/bundle.js
