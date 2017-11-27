from setuptools import setup

setup(name='skydive-client',
      version='0.1.0',
      description='Skydive Python 3 client library',
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
      ],
      zip_safe=False)
