const CopyWebpackPlugin = require('copy-webpack-plugin');
const WebpackBabelExternalsPlugin = require('webpack-babel-external-helpers-2');
const path = require('path');

module.exports = {
    entry: [
        path.resolve(__dirname, 'src/index.ts'),
    ],
    output: {
        filename: 'topologypackage.js',
        path: path.resolve(__dirname, '../components/')
    },
    resolve: {
        extensions: ['.tsx', '.ts', '.js'],
        modules: [
            path.resolve(__dirname, 'node_modules'),
            path.resolve(__dirname, 'bower_components'),
        ]
    },
    module: {
        rules: [
            {
                test: /\.js$/,
                use: 'babel-loader',
                exclude: [/node_modules\/(?!polymer-webpack-loader\/).*/]
            },
            {
                test: /\.tsx?$/,
                use: 'ts-loader'
            }
        ]
    },
    plugins: [
        new WebpackBabelExternalsPlugin(),
    ]
};
