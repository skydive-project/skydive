from setuptools import setup
import sys


def get_install_requires():
    install_requires = ['autobahn>=0.17.1', 'requests>=2.14.2']
    if sys.version_info[0] < 3:
        install_requires.append('trollius>=2.1')
    else:
        install_requires.append('asyncio>=3.4.3')
    return install_requires


setup(name='skydive-client',
      version='0.10.0',
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
      install_requires=get_install_requires(),
      zip_safe=False)
