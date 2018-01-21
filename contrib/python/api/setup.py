from setuptools import setup

setup(name='skydive-client',
      version='0.3.0',
      description='Skydive Python client library',
      url='http://github.com/skydive-project/skydive',
      author='Sylvain Afchain',
      author_email='safchain@gmail.com',
      license='Apache2',
      packages=['skydive', 'skydive.rest', 'skydive.websocket'],
      entry_points={
        'console_scripts': [
            'skydive-ws-client = skydive.wsshell:main',
        ],
      },
      install_requires=[
          'autobahn>=0.17.1',
          'trollius>=2.1;python_version<"3.0"',
          'asyncio>=3.4.3;python_version>="3.0"',
      ],
      zip_safe=False)
