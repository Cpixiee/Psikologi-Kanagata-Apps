// Navigation scroll effect
const navbar = document.getElementById('navbar');
const navLinks = document.querySelectorAll('.nav-link');
const navTexts = document.querySelectorAll('.nav-text');
const navLogos = document.querySelectorAll('.nav-logo');

let lastScroll = 0;

window.addEventListener('scroll', () => {
    const currentScroll = window.pageYOffset;
    
    if (currentScroll > 80) {
        navbar.classList.add('bg-white/95', 'backdrop-blur-md', 'shadow-lg');
        navbar.classList.remove('bg-transparent');
        
        // Change nav links to dark text
        navLinks.forEach(link => {
            link.classList.remove('text-white', 'hover:text-blue-200');
            link.classList.add('text-gray-900', 'hover:text-blue-600');
        });
        
        // Change nav text to dark
        navTexts.forEach(text => {
            text.classList.remove('text-white');
            text.classList.add('text-gray-900');
        });
    } else {
        navbar.classList.remove('bg-white/95', 'backdrop-blur-md', 'shadow-lg');
        navbar.classList.add('bg-transparent');
        
        // Change nav links back to white
        navLinks.forEach(link => {
            link.classList.add('text-white', 'hover:text-blue-200');
            link.classList.remove('text-gray-900', 'hover:text-blue-600');
        });
        
        // Change nav text back to white
        navTexts.forEach(text => {
            text.classList.add('text-white');
            text.classList.remove('text-gray-900');
        });
    }
    
    lastScroll = currentScroll;
});

// Smooth scroll for anchor links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            target.scrollIntoView({
                behavior: 'smooth',
                block: 'start'
            });
        }
    });
});

// Animate elements on scroll
const observerOptions = {
    threshold: 0.1,
    rootMargin: '0px 0px -50px 0px'
};

const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.classList.add('animate-fade-in');
        }
    });
}, observerOptions);

// Observe all cards and sections
document.querySelectorAll('section > div > div').forEach(el => {
    observer.observe(el);
});

// Interactive background elements animation
const backgroundElements = document.getElementById('backgroundElements');
if (backgroundElements) {
    let mouseX = 0;
    let mouseY = 0;
    let currentX = 0;
    let currentY = 0;
    
    document.addEventListener('mousemove', (e) => {
        mouseX = (e.clientX / window.innerWidth - 0.5) * 20;
        mouseY = (e.clientY / window.innerHeight - 0.5) * 20;
    });
    
    function animate() {
        currentX += (mouseX - currentX) * 0.05;
        currentY += (mouseY - currentY) * 0.05;
        
        if (backgroundElements) {
            backgroundElements.style.transform = `translate(${currentX}px, ${currentY}px) rotate(${currentX * 0.1}deg)`;
        }
        
        requestAnimationFrame(animate);
    }
    animate();
}

// Phone image interactive animation
const phoneImage = document.getElementById('phoneImage');
if (phoneImage) {
    let phoneMouseX = 0;
    let phoneMouseY = 0;
    let phoneCurrentX = 0;
    let phoneCurrentY = 0;
    
    const phoneContainer = phoneImage.closest('div');
    
    phoneContainer.addEventListener('mousemove', (e) => {
        const rect = phoneContainer.getBoundingClientRect();
        phoneMouseX = ((e.clientX - rect.left) / rect.width - 0.5) * 15;
        phoneMouseY = ((e.clientY - rect.top) / rect.height - 0.5) * 15;
    });
    
    phoneContainer.addEventListener('mouseleave', () => {
        phoneMouseX = 0;
        phoneMouseY = 0;
    });
    
    function animatePhone() {
        phoneCurrentX += (phoneMouseX - phoneCurrentX) * 0.1;
        phoneCurrentY += (phoneMouseY - phoneCurrentY) * 0.1;
        
        if (phoneImage) {
            phoneImage.style.transform = `translate(${phoneCurrentX}px, ${phoneCurrentY}px) rotateY(${phoneCurrentX * 0.5}deg) rotateX(${-phoneCurrentY * 0.5}deg)`;
        }
        
        requestAnimationFrame(animatePhone);
    }
    animatePhone();
}

// Parallax effect for hero section
window.addEventListener('scroll', () => {
    const scrolled = window.pageYOffset;
    const heroSection = document.getElementById('home');
    if (heroSection) {
        const heroContent = heroSection.querySelector('.container');
        if (heroContent && scrolled < window.innerHeight) {
            heroContent.style.transform = `translateY(${scrolled * 0.3}px)`;
            heroContent.style.opacity = 1 - (scrolled / window.innerHeight) * 0.5;
        }
    }
});

// Add CSS animations via JavaScript
const style = document.createElement('style');
style.textContent = `
    @keyframes float {
        0%, 100% {
            transform: translateY(0px) rotate(0deg);
        }
        50% {
            transform: translateY(-20px) rotate(2deg);
        }
    }
    
    @keyframes phone-float {
        0%, 100% {
            transform: translateY(0px) rotateY(0deg);
        }
        50% {
            transform: translateY(-15px) rotateY(5deg);
        }
    }
    
    @keyframes fade-in {
        from {
            opacity: 0;
            transform: translateY(30px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }
    
    .animate-float {
        animation: float 6s ease-in-out infinite;
    }
    
    .animate-phone-float {
        animation: phone-float 4s ease-in-out infinite;
    }
    
    .animate-fade-in {
        animation: fade-in 0.8s ease-out forwards;
    }
    
    /* Smooth transitions for nav */
    #navbar {
        transition: background-color 0.3s ease, box-shadow 0.3s ease;
    }
    
    .nav-link {
        transition: color 0.3s ease;
    }
    
    .nav-text {
        transition: color 0.3s ease;
    }
    
    /* Responsive adjustments */
    @media (max-width: 768px) {
        .animate-float {
            animation-duration: 4s;
        }
        
        .animate-phone-float {
            animation-duration: 3s;
        }
    }
`;
document.head.appendChild(style);
